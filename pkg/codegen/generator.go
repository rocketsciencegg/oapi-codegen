package codegen

import (
	"maps"
	"reflect"
	"sync"
	"sync/atomic"
	"text/template"

	"github.com/getkin/kin-openapi/openapi3"
)

// Generator holds generation configuration and template hooks. It may be reused
// concurrently with independently loaded specifications. Configuration values
// and hook implementations must not be mutated while generation is running.
// Custom hooks must be safe for concurrent calls when the generator is shared.
type Generator struct {
	config    Configuration
	functions template.FuncMap

	mu   sync.RWMutex
	spec *openapi3.T
}

var defaultGenerator atomic.Pointer[Generator]

// init installs the default generator before package-level helpers are used.
func init() {
	SetDefault(NewGenerator())
}

// Default returns the generator used by package-level functions.
func Default() *Generator {
	return defaultGenerator.Load()
}

// SetDefault sets the generator used by subsequent package-level calls.
// It is safe to call concurrently and panics if g is nil.
// Calls already in progress retain their generation state.
func SetDefault(g *Generator) {
	if g == nil {
		panic("codegen: nil Generator")
	}
	defaultGenerator.Store(g)
}

// GeneratorOption customizes a generator during construction.
type GeneratorOption func(*Generator)

// WithConfig sets the generator's configuration, replacing any
// configuration supplied by an earlier option.
func WithConfig(config Configuration) GeneratorOption {
	return func(g *Generator) {
		g.config = config
	}
}

// WithTemplateFuncs adds per-instance template hooks, overriding built-in
// functions and earlier options with the same names. The supplied map is copied.
func WithTemplateFuncs(functions template.FuncMap) GeneratorOption {
	functions = maps.Clone(functions)
	return func(g *Generator) {
		if g.functions == nil {
			g.functions = make(template.FuncMap)
		}
		maps.Copy(g.functions, functions)
	}
}

// NewGenerator constructs a generator with zero-value configuration and applies
// the supplied options in order.
func NewGenerator(options ...GeneratorOption) *Generator {
	g := &Generator{}
	for _, option := range options {
		option(g)
	}
	return g
}

// Generate generates code with the generator's configuration. Generation may
// filter and prune spec, so callers must not share a spec between invocations.
func (g *Generator) Generate(spec *openapi3.T) (string, error) {
	return g.newState().Generate(spec)
}

// Generate generates code using the default generator's hooks and the supplied
// configuration. Each call owns its state and may run concurrently.
// It does not change the default generator's configuration. Generation may
// filter and prune spec, so concurrent calls must use independently loaded
// specifications. Configuration values must not be mutated during generation.
func Generate(spec *openapi3.T, options Configuration) (string, error) {
	return Default().newStateWithConfig(options).Generate(spec)
}

// SetGlobalStateSpec sets the default specification used by package-level
// helpers. It does not affect Generate or existing generated model values.
// It applies only to the current default generator; call it after SetDefault.
// Replacing the default does not carry the specification to the new generator.
//
// Deprecated: use a Generator for generation instead of implicit helper state.
func SetGlobalStateSpec(spec *openapi3.T) {
	g := Default()
	g.mu.Lock()
	defer g.mu.Unlock()
	g.spec = spec
}

// newDefaultState creates isolated run state from the current default generator.
func newDefaultState() *state {
	return Default().newState()
}

// generationOrDefault preserves a model's owning run, or creates a default run
// for model values constructed directly by callers.
func generationOrDefault(gen *state) *state {
	if gen != nil {
		return gen
	}
	return newDefaultState()
}

// newState creates isolated generation state from the generator's configuration and
// snapshots the legacy helper specification under its lock.
func (g *Generator) newState() *state {
	return g.newStateWithConfig(g.config)
}

// newStateWithConfig creates isolated state using an explicit configuration.
func (g *Generator) newStateWithConfig(config Configuration) *state {
	g.mu.RLock()
	spec := g.spec
	g.mu.RUnlock()

	gen := &state{
		options:            config,
		spec:               spec,
		typeMapping:        DefaultTypeMapping,
		importMapping:      constructImportMapping(config.ImportMapping),
		nameNormalizer:     ToCamelCase,
		responseTypeSuffix: "Response",
		templateOverrides:  g.functions,
	}

	gen.nameNormalizer = gen.normalizerFor(NameNormalizerFunction(config.OutputOptions.NameNormalizer))
	if spec != nil {
		gen.is31 = spec.IsOpenAPI31OrLater()
	}

	if suffix := config.OutputOptions.ResponseTypeSuffix; suffix != "" {
		gen.responseTypeSuffix = suffix
	}

	if mapping := config.OutputOptions.TypeMapping; mapping != nil {
		gen.typeMapping = DefaultTypeMapping.Merge(*mapping)
	}

	return gen
}

// normalizerFor binds the built-in initialism normalizer to this run while
// preserving custom normalizers registered by callers.
func (s *state) normalizerFor(name NameNormalizerFunction) NameNormalizer {
	normalizer := NameNormalizers[name]
	if normalizer != nil && reflect.ValueOf(normalizer).Pointer() == reflect.ValueOf(ToCamelCaseWithInitialisms).Pointer() {
		return s.ToCamelCaseWithInitialisms
	}
	return normalizer
}

// functions copies the legacy registry, binds stateful built-ins to this run,
// and applies per-generator overrides without modifying the shared registry.
func (s *state) functions(overrides template.FuncMap) template.FuncMap {
	functions := maps.Clone(TemplateFunctions)
	bind := func(name string, original, bound any) {
		// Preserve legacy customizations made to TemplateFunctions before use.
		if fn, ok := functions[name]; ok && reflect.ValueOf(fn).Pointer() == reflect.ValueOf(original).Pointer() {
			functions[name] = bound
		}
	}
	bind("schemaNameToTypeName", SchemaNameToTypeName, s.SchemaNameToTypeName)
	bind("genResponsePayload", genResponsePayload, s.genResponsePayload)
	bind("genResponseTypeName", genResponseTypeName, s.genResponseTypeName)
	maps.Copy(functions, overrides)
	functions["opts"] = func() Configuration {
		return s.options
	}

	return functions
}
