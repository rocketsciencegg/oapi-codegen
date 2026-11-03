package codegen

import (
	"text/template"

	"github.com/getkin/kin-openapi/openapi3"
)

func MergeSchemas(allOf []*openapi3.SchemaRef, path []string) (Schema, error) {
	return newDefaultState().MergeSchemas(allOf, path)
}

func mergeSchemas(allOf []*openapi3.SchemaRef, path []string) (Schema, error) {
	return newDefaultState().mergeSchemas(allOf, path)
}

func GenStructFromAllOf(allOf []*openapi3.SchemaRef, path []string) (string, error) {
	return newDefaultState().GenStructFromAllOf(allOf, path)
}

func mergeAllOf(allOf []*openapi3.SchemaRef, seenSchemaRef map[string]bool) (openapi3.Schema, error) {
	return newDefaultState().mergeAllOf(allOf, seenSchemaRef)
}

func mergeOpenapiSchemas(s1, s2 openapi3.Schema, allOf bool, seenSchemaRef map[string]bool) (openapi3.Schema, error) {
	return newDefaultState().mergeOpenapiSchemas(s1, s2, allOf, seenSchemaRef)
}

func generateUnion(outSchema *Schema, elements openapi3.SchemaRefs, discriminator *openapi3.Discriminator, path []string) error {
	return generationOrDefault(outSchema.gen).generateUnion(outSchema, elements, discriminator, path)
}

func allOfMergesUnions() bool {
	return newDefaultState().allOfMergesUnions()
}

// genResponsePayload generates the payload returned at the end of each client request function
func genResponsePayload(operationID string) string {
	return newDefaultState().genResponsePayload(operationID)
}

// genResponseTypeName creates the name of generated response types (given the operationID).
// It first checks if the multi-pass name resolver has assigned a name for this
// wrapper type (which would happen if the default name collides with a schema type).
func genResponseTypeName(operationID string) string {
	return newDefaultState().genResponseTypeName(operationID)
}

// GenerateConstants generates operation ids, context keys, paths, etc. to be exported as constants
//
// Scopes constants are derived from components/securitySchemes rather than
// from the operations' security requirements (which are filtered to defined
// schemes anyway), so that a spec holding shared definitions with no paths
// still exports the constants other packages alias via import-mapping.
func GenerateConstants(t *template.Template, swagger *openapi3.T) (string, error) {
	return newDefaultState().GenerateConstants(t, swagger)
}

// GenerateTypesForSchemas generates type definitions for any custom types defined in the
// components/schemas section of the Swagger spec.
func GenerateTypesForSchemas(t *template.Template, schemas map[string]*openapi3.SchemaRef, excludeSchemas []string) ([]TypeDefinition, error) {
	return newDefaultState().GenerateTypesForSchemas(t, schemas, excludeSchemas)
}

// GenerateTypesForParameters generates type definitions for any custom types defined in the
// components/parameters section of the Swagger spec.
func GenerateTypesForParameters(t *template.Template, params map[string]*openapi3.ParameterRef) ([]TypeDefinition, error) {
	return newDefaultState().GenerateTypesForParameters(t, params)
}

// GenerateTypesForResponses generates type definitions for any custom types defined in the
// components/responses section of the Swagger spec.
func GenerateTypesForResponses(t *template.Template, responses openapi3.ResponseBodies) ([]TypeDefinition, error) {
	return newDefaultState().GenerateTypesForResponses(t, responses)
}

// GenerateTypesForRequestBodies generates type definitions for any custom types defined in the
// components/requestBodies section of the Swagger spec.
func GenerateTypesForRequestBodies(t *template.Template, bodies map[string]*openapi3.RequestBodyRef) ([]TypeDefinition, error) {
	return newDefaultState().GenerateTypesForRequestBodies(t, bodies)
}

// GenerateTypesForSecuritySchemes generates type definitions for any custom types defined in the
// components/securitySchemes section of the Swagger spec.
func GenerateTypesForSecuritySchemes(t *template.Template, schemes map[string]*openapi3.SecuritySchemeRef) ([]TypeDefinition, error) {
	return newDefaultState().GenerateTypesForSecuritySchemes(t, schemes)
}

// GenerateEnums renders enum constants and validation methods for the given types.
func GenerateEnums(t *template.Template, types []TypeDefinition) (string, error) {
	return newDefaultState().GenerateEnums(t, types)
}

// GenerateImports generates our import statements and package definition.
func GenerateImports(t *template.Template, externalImports []string, packageName string, versionOverride *string) (string, error) {
	return newDefaultState().GenerateImports(t, externalImports, packageName, versionOverride)
}

// OperationSchemaImports collects custom Go imports from an operation schema and its properties.
func OperationSchemaImports(s *Schema) (map[string]goImport, error) {
	return newDefaultState().OperationSchemaImports(s)
}

// OperationImports collects custom Go imports from operation parameters, bodies, and responses.
func OperationImports(ops []OperationDefinition) (map[string]goImport, error) {
	return newDefaultState().OperationImports(ops)
}

// GetTypeDefinitionsImports collects custom Go imports from component type definitions.
func GetTypeDefinitionsImports(swagger *openapi3.T, excludeSchemas []string) (map[string]goImport, error) {
	return newDefaultState().GetTypeDefinitionsImports(swagger, excludeSchemas)
}

// GoSchemaImports collects x-go-type-import declarations from schemas and their nested properties and items.
func GoSchemaImports(schemas ...*openapi3.SchemaRef) (map[string]goImport, error) {
	return newDefaultState().GoSchemaImports(schemas...)
}

// GetSchemaImports collects custom Go imports from schemas not listed in excludeSchemas.
func GetSchemaImports(schemas map[string]*openapi3.SchemaRef, excludeSchemas []string) (map[string]goImport, error) {
	return newDefaultState().GetSchemaImports(schemas, excludeSchemas)
}

// GetRequestBodiesImports collects custom Go imports from JSON request body schemas.
func GetRequestBodiesImports(bodies map[string]*openapi3.RequestBodyRef) (map[string]goImport, error) {
	return newDefaultState().GetRequestBodiesImports(bodies)
}

// GetResponsesImports collects custom Go imports from JSON response schemas.
func GetResponsesImports(responses map[string]*openapi3.ResponseRef) (map[string]goImport, error) {
	return newDefaultState().GetResponsesImports(responses)
}

// GetParametersImports collects custom Go imports from parameter schemas.
func GetParametersImports(params map[string]*openapi3.ParameterRef) (map[string]goImport, error) {
	return newDefaultState().GetParametersImports(params)
}

// BuildServerURLTypeDefinitions synthesizes a TypeDefinition for every
// server-URL variable that defines an `enum`. These are appended into
// the same TypeDefinition slices used by GenerateTypes (typedef.tmpl)
// and GenerateEnums (constants.tmpl), so that server-URL enum
// variables get the same `type X string`, `const ( … )` block, and
// `Valid()` method as any other enum-bearing schema. The
// server-urls.tmpl template no longer emits these declarations
// directly.
//
// Variables without an `enum` are not handled here: server-urls.tmpl
// continues to emit their `type` and (optional) default constant
// inline, since the generic enum path has nothing to contribute for a
// non-enum string.
func BuildServerURLTypeDefinitions(spec *openapi3.T) ([]TypeDefinition, error) {
	return newDefaultState().BuildServerURLTypeDefinitions(spec)
}

// GenerateServerURLs renders server URL constants and variable expansion helpers.
func GenerateServerURLs(t *template.Template, spec *openapi3.T) (string, error) {
	return newDefaultState().GenerateServerURLs(t, spec)
}

// ResolveNames takes the gathered schemas and assigns unique Go type names to each.
// It returns a map from the schema's path string to the resolved Go type name.
func ResolveNames(schemas []*GatheredSchema) map[string]string {
	return newDefaultState().ResolveNames(schemas)
}

// DescribeParameters walks the given parameters dictionary, and generates the above
// descriptors into a flat list. This makes it a lot easier to traverse the
// data in the template engine.
func DescribeParameters(params openapi3.Parameters, path []string) ([]ParameterDefinition, error) {
	return newDefaultState().DescribeParameters(params, path)
}

// OperationDefinitions describes path operations for code generation.
func OperationDefinitions(swagger *openapi3.T) ([]OperationDefinition, error) {
	return newDefaultState().OperationDefinitions(swagger)
}

// WebhookOperationDefinitions extracts OpenAPI 3.1+ webhook operations
// from swagger.Webhooks into the same OperationDefinition shape used for
// path operations, so they flow through the same downstream pipeline
// (body / response generation, type definitions, etc.) but are routed
// to webhook-specific templates.
//
// kin-openapi only populates the Webhooks field for OpenAPI 3.1+
// documents, so a missing/empty map naturally short-circuits this for
// 3.0 specs without an explicit version check.
//
// The result mirrors OperationDefinitions in structure, minus the
// path-alias logic (webhooks have no path template) and the path-
// parameter extraction (webhooks have no path params).
func WebhookOperationDefinitions(swagger *openapi3.T) ([]OperationDefinition, error) {
	return newDefaultState().WebhookOperationDefinitions(swagger)
}

// CallbackOperationDefinitions extracts OpenAPI callback operations
// from spec.Paths.<path>.<method>.Callbacks into the same
// OperationDefinition shape used for path operations, so they flow
// through the same downstream pipeline (body / response generation,
// type definitions, etc.) but are routed to callback-specific
// templates.
//
// Callbacks have been part of the OpenAPI spec since 3.0, so this is
// not gated on version: any spec that declares callbacks gets them
// generated. The spec shape:
//
//	paths:
//	  /api/plant_tree:
//	    post:
//	      operationId: PlantTree
//	      callbacks:
//	        treePlanted:                 # the callback map key
//	          '{$request.body#/url}':    # the runtime URL expression
//	            post:
//	              operationId: TreePlanted
//
// Each leaf operation (the inner `post:` above) becomes one
// OperationDefinition with IsCallback=true and CallbackName set to the
// outer map key ("treePlanted"). The codegen does not interpret the URL
// expression itself -- the caller of the generated CallbackInitiator
// supplies the resolved target URL at runtime (the same way it would
// for a webhook).
func CallbackOperationDefinitions(swagger *openapi3.T) ([]OperationDefinition, error) {
	return newDefaultState().CallbackOperationDefinitions(swagger)
}

// GenerateBodyDefinitions turns the Swagger body definitions into a list of our body
// definitions which will be used for code generation.
//
// pathItemRef is the path item's $ref (if any). When non-empty and pointing at
// an external file, the body type that would otherwise be hoisted locally is
// replaced by a reference to the imported package's same-named type — the
// imported package already declares it (with any As/From/Merge methods), so
// redeclaring locally would just produce an awkward duplicate with
// package-qualified union elements.
func GenerateBodyDefinitions(operationID string, bodyOrRef *openapi3.RequestBodyRef, pathItemRef string) ([]RequestBodyDefinition, []TypeDefinition, error) {
	return newDefaultState().GenerateBodyDefinitions(operationID, bodyOrRef, pathItemRef)
}

// GenerateResponseDefinitions describes an operation's response schemas, headers, and references.
func GenerateResponseDefinitions(operationID string, responses map[string]*openapi3.ResponseRef, pathItemRef string) ([]ResponseDefinition, error) {
	return newDefaultState().GenerateResponseDefinitions(operationID, responses, pathItemRef)
}

// GenerateTypeDefsForOperation collects parameter types and auxiliary body and response types.
func GenerateTypeDefsForOperation(op OperationDefinition) []TypeDefinition {
	return newDefaultState().GenerateTypeDefsForOperation(op)
}

// GenerateParamsTypes defines the schema for a parameters definition object
// which encapsulates all the query, header and cookie parameters for an operation.
func GenerateParamsTypes(op OperationDefinition) []TypeDefinition {
	return newDefaultState().GenerateParamsTypes(op)
}

// GenerateIrisServer generates all the go code for the ServerInterface as well as
// all the wrapper functions around our handlers.
func GenerateIrisServer(t *template.Template, operations []OperationDefinition) (string, error) {
	return newDefaultState().GenerateIrisServer(t, operations)
}

// GenerateChiServer generates all the go code for the ServerInterface as well as
// all the wrapper functions around our handlers.
func GenerateChiServer(t *template.Template, operations []OperationDefinition) (string, error) {
	return newDefaultState().GenerateChiServer(t, operations)
}

// GenerateFiberServer generates all the go code for the ServerInterface as well as
// all the wrapper functions around our handlers.
func GenerateFiberServer(t *template.Template, operations []OperationDefinition) (string, error) {
	return newDefaultState().GenerateFiberServer(t, operations)
}

// GenerateFiberV3Server generates all the go code for the ServerInterface as well as
// all the wrapper functions around our handlers.
func GenerateFiberV3Server(t *template.Template, operations []OperationDefinition) (string, error) {
	return newDefaultState().GenerateFiberV3Server(t, operations)
}

// GenerateEchoServer generates all the go code for the ServerInterface as well as
// all the wrapper functions around our handlers.
func GenerateEchoServer(t *template.Template, operations []OperationDefinition) (string, error) {
	return newDefaultState().GenerateEchoServer(t, operations)
}

// GenerateEcho5Server generates all the go code for the ServerInterface as well as
// all the wrapper functions around our handlers.
func GenerateEcho5Server(t *template.Template, operations []OperationDefinition) (string, error) {
	return newDefaultState().GenerateEcho5Server(t, operations)
}

// GenerateGinServer generates all the go code for the ServerInterface as well as
// all the wrapper functions around our handlers.
func GenerateGinServer(t *template.Template, operations []OperationDefinition) (string, error) {
	return newDefaultState().GenerateGinServer(t, operations)
}

// GenerateGorillaServer generates all the go code for the ServerInterface as well as
// all the wrapper functions around our handlers.
func GenerateGorillaServer(t *template.Template, operations []OperationDefinition) (string, error) {
	return newDefaultState().GenerateGorillaServer(t, operations)
}

// GenerateStdHTTPServer generates all the go code for the ServerInterface as well as
// all the wrapper functions around our handlers.
func GenerateStdHTTPServer(t *template.Template, operations []OperationDefinition) (string, error) {
	return newDefaultState().GenerateStdHTTPServer(t, operations)
}

// GenerateGoSchema converts an OpenAPI schema into a Go type definition.
//
// It starts a fresh generation context, so a call made from outside the
// package behaves exactly as it always has. Within the package, prefer
// generateGoSchema so that the recursion state described by genContext
// survives the descent.
func GenerateGoSchema(sref *openapi3.SchemaRef, path []string) (Schema, error) {
	return newDefaultState().GenerateGoSchema(sref, path)
}

// GenFieldsFromProperties produce corresponding field names with JSON annotations,
// given a list of schema descriptors
func GenFieldsFromProperties(props []Property) []string {
	return newDefaultState().GenFieldsFromProperties(props)
}

// GenStructFromSchema renders a Go struct declaration from a schema.
func GenStructFromSchema(schema Schema) string {
	return newDefaultState().GenStructFromSchema(schema)
}

// ToCamelCaseWithInitialisms function will convert query-arg style strings to CamelCase with initialisms in uppercase.
// So, httpOperationId would be converted to HTTPOperationID
func ToCamelCaseWithInitialisms(s string) string {
	return newDefaultState().ToCamelCaseWithInitialisms(s)
}

// ToCamelCaseWithInitialism converts str to camel case and capitalizes recognized initialisms.
func ToCamelCaseWithInitialism(str string) string {
	return newDefaultState().ToCamelCaseWithInitialism(str)
}

// RefPathToGoType takes a $ref value and converts it to a Go typename.
// #/components/schemas/Foo -> Foo
// #/components/parameters/Bar -> Bar
// #/components/responses/Baz -> Baz
// Remote components (document.json#/Foo) are supported if they present in --import-mapping
// URL components (http://deepmap.com/schemas/document.json#/Foo) are supported if they present in --import-mapping
// Remote and URL also support standard local paths even though the spec doesn't mention them.
func RefPathToGoType(refPath string) (string, error) {
	return newDefaultState().RefPathToGoType(refPath)
}

// SanitizeEnumNames fixes illegal chars in the enum names
// and removes duplicates
func SanitizeEnumNames(enumNames, enumValues []string) map[string]string {
	return newDefaultState().SanitizeEnumNames(enumNames, enumValues)
}

// SchemaNameToTypeName converts a Schema name to a valid Go type name. It converts to camel case, and makes sure the name is
// valid in Go
func SchemaNameToTypeName(name string) string {
	return newDefaultState().SchemaNameToTypeName(name)
}

// PathToTypeName converts a path, like Object/field1/nestedField into a go
// type name.
func PathToTypeName(path []string) string { return newDefaultState().PathToTypeName(path) }
