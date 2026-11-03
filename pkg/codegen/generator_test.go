package codegen

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"text/template"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

const isolatedGenerationSpec = `
openapi: %s
info:
  title: Isolation
  version: "1"
paths:
  /records:
    get:
      operationId: getHttpRecord
      parameters:
        - in: query
          name: httpId
          schema:
            type: string
      responses:
        "200":
          description: A record
          headers:
            X-Value:
              schema:
                type: string
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Record"
            application/x-isolated-stream:
              schema:
                type: string
components:
  schemas:
    Record:
      allOf:
        - type: object
          properties:
            httpId:
              type: string
            widgetId:
              type: string
        - type: object
          properties:
            count:
              type: integer
`

func loadIsolatedGenerationSpec(t *testing.T, version string) *openapi3.T {
	t.Helper()
	spec, err := openapi3.NewLoader().LoadFromData([]byte(fmt.Sprintf(isolatedGenerationSpec, version)))
	require.NoError(t, err)
	return spec
}

func TestGenerateIsolation(t *testing.T) {
	base := Configuration{
		PackageName: "isolation",
		Generate:    GenerateOptions{Models: true, Client: true, StdHTTPServer: true, Strict: true},
	}
	custom := base
	custom.OutputOptions = OutputOptions{
		NameNormalizer:                   string(NameNormalizerFunctionToCamelCaseWithInitialisms),
		AdditionalInitialisms:            []string{"WIDGET"},
		ResponseTypeSuffix:               "Result",
		ClientTypeName:                   "RecordClient",
		NullableType:                     true,
		ResolveTypeNameCollisions:        true,
		GenerateTypesForAnonymousSchemas: true,
		StreamingContentTypes:            []string{"^application/x-isolated-stream$"},
		TypeMapping:                      &TypeMapping{Integer: FormatMapping{Default: SimpleTypeSpec{Type: "int64"}}},
	}
	want, err := Generate(loadIsolatedGenerationSpec(t, "3.0.3"), base)
	require.NoError(t, err)
	require.Contains(t, want, "GetHttpRecordResponse")
	require.Contains(t, want, "HttpId")
	customCode, err := Generate(loadIsolatedGenerationSpec(t, "3.1.0"), custom)
	require.NoError(t, err)
	require.Contains(t, customCode, "GetHTTPRecordResult")
	require.Contains(t, customCode, "WIDGETID")
	require.Contains(t, customCode, "*int64")
	require.Contains(t, customCode, "type RecordClient struct")
	again, err := Generate(loadIsolatedGenerationSpec(t, "3.0.3"), base)
	require.NoError(t, err)
	require.Equal(t, want, again, "a prior invocation must not change default output")

	for _, version := range []bool{false, true} {
		for _, options := range []Configuration{base, custom} {
			options.Compatibility.OldMergeSchemas = version
			for _, openapiVersion := range []string{"3.0.3", "3.1.0"} {
				t.Run(fmt.Sprint(version)+"/"+openapiVersion+"/"+options.OutputOptions.ClientTypeName, func(t *testing.T) {
					t.Parallel()
					generator := NewGenerator(WithConfig(options))
					expected, err := generator.Generate(loadIsolatedGenerationSpec(t, openapiVersion))
					require.NoError(t, err)
					for i := range 3 {
						t.Run(fmt.Sprint(i), func(t *testing.T) {
							t.Parallel()
							got, err := generator.Generate(loadIsolatedGenerationSpec(t, openapiVersion))
							require.NoError(t, err)
							require.Equal(t, expected, got)
							got, err = Generate(loadIsolatedGenerationSpec(t, openapiVersion), options)
							require.NoError(t, err)
							require.Equal(t, expected, got)
						})
					}
				})
			}
		}
	}
}

func TestGenerateImportMappingIsolation(t *testing.T) {
	for _, path := range []string{"example.com/first", "example.com/second"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			options := Configuration{
				PackageName:   "imports",
				Generate:      GenerateOptions{Models: true},
				ImportMapping: map[string]string{"other.yaml": "other " + path},
			}
			for range 3 {
				spec := loadIsolatedGenerationSpec(t, "3.0.3")
				spec.Components.Schemas["Record"].Value.AllOf[0].Value.Properties["external"] = &openapi3.SchemaRef{
					Ref:   "other.yaml#/components/schemas/External",
					Value: openapi3.NewStringSchema(),
				}
				code, err := Generate(spec, options)
				require.NoError(t, err)
				require.Contains(t, code, `other "`+path+`"`)
				require.Contains(t, code, "*other.External")
			}
		})
	}
}

func TestGenerateAfterFailure(t *testing.T) {
	options := Configuration{PackageName: "failure", Generate: GenerateOptions{Models: true}}
	want, err := Generate(loadIsolatedGenerationSpec(t, "3.0.3"), options)
	require.NoError(t, err)
	for _, outputOptions := range []OutputOptions{
		{NameNormalizer: "missing"},
		{ResponseTypeSuffix: "Custom", UserTemplates: map[string]string{"typedef.tmpl": "{{missingFunction}}"}},
		{StreamingContentTypes: []string{"["}},
	} {
		invalid := options
		invalid.OutputOptions = outputOptions
		_, err := Generate(loadIsolatedGenerationSpec(t, "3.0.3"), invalid)
		require.Error(t, err)
		got, err := Generate(loadIsolatedGenerationSpec(t, "3.0.3"), options)
		require.NoError(t, err)
		require.Equal(t, want, got)
	}
}

func TestLegacyRegistries(t *testing.T) {
	normalizer := NameNormalizers[NameNormalizerFunctionToCamelCaseWithInitialisms]
	responseName := TemplateFunctions["genResponseTypeName"]
	t.Cleanup(func() {
		NameNormalizers[NameNormalizerFunctionToCamelCaseWithInitialisms] = normalizer
		TemplateFunctions["genResponseTypeName"] = responseName
	})
	NameNormalizers[NameNormalizerFunctionToCamelCaseWithInitialisms] = func(s string) string {
		return "Custom" + ToCamelCase(s)
	}
	TemplateFunctions["genResponseTypeName"] = func(s string) string { return s + "Legacy" }
	count := len(TemplateFunctions)
	code, err := Generate(loadIsolatedGenerationSpec(t, "3.0.3"), Configuration{
		PackageName:   "legacy",
		Generate:      GenerateOptions{Models: true, Client: true},
		OutputOptions: OutputOptions{NameNormalizer: string(NameNormalizerFunctionToCamelCaseWithInitialisms)},
	})
	require.NoError(t, err)
	require.Contains(t, code, "CustomGetHttpRecordLegacy")
	require.Len(t, TemplateFunctions, count, "generation must not insert opts into the shared map")
}

func TestGeneratorTemplateHooks(t *testing.T) {
	for _, marker := range []string{"first", "second"} {
		t.Run(marker, func(t *testing.T) {
			t.Parallel()
			options := Configuration{
				PackageName: "hooks",
				Generate:    GenerateOptions{Models: true},
				OutputOptions: OutputOptions{
					UserTemplates: map[string]string{
						"typedef.tmpl": `{{range .Types}}// {{marker}} {{title "a title"}}
type {{.TypeName}} {{.Schema.TypeDecl}}
{{end}}`,
					},
				},
			}
			functions := template.FuncMap{"marker": func() string { return marker }}
			generator := NewGenerator(WithConfig(options), WithTemplateFuncs(functions))
			functions["marker"] = func() string { return "changed" }
			for range 3 {
				code, err := generator.Generate(loadIsolatedGenerationSpec(t, "3.0.3"))
				require.NoError(t, err)
				require.Contains(t, code, "// "+marker+" A Title")
				require.NotContains(t, code, "changed")
			}
		})
	}
}

func TestGeneratedModelsRetainRunSettings(t *testing.T) {
	options := Configuration{OutputOptions: OutputOptions{NullableType: true}}
	gen := NewGenerator(WithConfig(options)).newState()
	schema := &openapi3.SchemaRef{Value: openapi3.NewObjectSchema().WithProperty("value", openapi3.NewStringSchema().WithNullable())}
	model, err := gen.GenerateGoSchema(schema, []string{"Record"})
	require.NoError(t, err)
	require.Len(t, model.Properties, 1)
	before := model.Properties[0].GoTypeDef()
	require.Equal(t, "nullable.Nullable[string]", before)
	_, err = Generate(loadIsolatedGenerationSpec(t, "3.0.3"), Configuration{PackageName: "other", Generate: GenerateOptions{Models: true}})
	require.NoError(t, err)
	require.Equal(t, before, model.Properties[0].GoTypeDef())
}

func TestStableModelsRetainRunSettings(t *testing.T) {
	previous := Default()
	t.Cleanup(func() { SetDefault(previous) })
	SetDefault(NewGenerator())
	gen := NewGenerator(WithConfig(Configuration{OutputOptions: OutputOptions{
		NameNormalizer: string(NameNormalizerFunctionToCamelCaseWithInitialisms),
		NullableType:   true,
	}})).newState()

	params, err := gen.DescribeParameters(openapi3.Parameters{{Value: &openapi3.Parameter{
		Name: "httpId", In: "query", Schema: &openapi3.SchemaRef{Value: openapi3.NewStringSchema()},
	}}}, nil)
	require.NoError(t, err)
	require.Equal(t, "HTTPID", params[0].GoName())

	responses, err := gen.GenerateResponseDefinitions("GetRecord", map[string]*openapi3.ResponseRef{
		"200": {Value: &openapi3.Response{Headers: openapi3.Headers{
			"Value": {Value: &openapi3.Header{Parameter: openapi3.Parameter{
				Schema: &openapi3.SchemaRef{Value: openapi3.NewStringSchema().WithNullable()},
			}}},
		}}},
	}, "")
	require.NoError(t, err)
	require.Equal(t, "nullable.Nullable[string]", responses[0].Headers[0].GoTypeDef())

	union, err := gen.GenerateGoSchema(&openapi3.SchemaRef{Value: &openapi3.Schema{
		OneOf: openapi3.SchemaRefs{
			{Ref: "#/components/schemas/First", Value: openapi3.NewObjectSchema()},
			{Ref: "#/components/schemas/Second", Value: openapi3.NewObjectSchema()},
		},
		Discriminator: &openapi3.Discriminator{PropertyName: "httpId"},
	}}, []string{"Union"})
	require.NoError(t, err)
	require.Equal(t, "HTTPID", union.Discriminator.PropertyName())

	SetDefault(NewGenerator(WithConfig(Configuration{OutputOptions: OutputOptions{
		NameNormalizer: string(NameNormalizerFunctionToCamelCase),
	}})))
	require.Equal(t, "HTTPID", params[0].GoName())
	require.Equal(t, "nullable.Nullable[string]", responses[0].Headers[0].GoTypeDef())
	require.Equal(t, "HTTPID", union.Discriminator.PropertyName())
}

func TestGeneratorOptions(t *testing.T) {
	functions := template.FuncMap{"marker": func() string { return "original" }}
	option := WithTemplateFuncs(functions)
	functions["marker"] = func() string { return "changed" }
	first := NewGenerator(option)
	second := NewGenerator(option,
		WithTemplateFuncs(template.FuncMap{"marker": func() string { return "override" }}),
		WithTemplateFuncs(template.FuncMap{"extra": func() string { return "extra" }}),
	)
	tmpl, err := template.New("hooks").Funcs(first.functions).Parse("{{marker}}")
	require.NoError(t, err)
	var firstOutput strings.Builder
	require.NoError(t, tmpl.Execute(&firstOutput, nil))
	require.Equal(t, "original", firstOutput.String())
	tmpl, err = template.New("hooks").Funcs(second.functions).Parse("{{marker}} {{extra}}")
	require.NoError(t, err)
	var secondOutput strings.Builder
	require.NoError(t, tmpl.Execute(&secondOutput, nil))
	require.Equal(t, "override extra", secondOutput.String())
}

func TestGeneratorConfigurationOption(t *testing.T) {
	require.Equal(t, Configuration{}, NewGenerator().config)
	first := Configuration{PackageName: "first", Generate: GenerateOptions{Models: true}}
	second := Configuration{PackageName: "second"}
	option := WithConfig(first)
	require.Equal(t, first, NewGenerator(option).config)
	require.Equal(t, second, NewGenerator(option, WithConfig(second)).config)
	require.Equal(t, first, NewGenerator(option).config, "options can be reused")
}

func TestDefaultGeneratorHelpers(t *testing.T) {
	previous := Default()
	t.Cleanup(func() { SetDefault(previous) })
	SetDefault(NewGenerator(WithConfig(Configuration{
		ImportMapping: map[string]string{"other.yaml": "other example.com/other"},
	})))
	SetGlobalStateSpec(&openapi3.T{Components: &openapi3.Components{
		Schemas: openapi3.Schemas{"Record": {Value: &openapi3.Schema{
			Extensions: map[string]any{extGoName: "Renamed"},
		}}},
	}})
	name, err := RefPathToGoType("#/components/schemas/Record")
	require.NoError(t, err)
	require.Equal(t, "Renamed", name)
	name, err = RefPathToGoType("other.yaml#/components/schemas/Record")
	require.NoError(t, err)
	require.Equal(t, "other.Renamed", name)
	_, err = Generate(loadIsolatedGenerationSpec(t, "3.0.3"), Configuration{PackageName: "other", Generate: GenerateOptions{Models: true}})
	require.NoError(t, err)
	name, err = RefPathToGoType("#/components/schemas/Record")
	require.NoError(t, err)
	require.Equal(t, "Renamed", name)
	require.Equal(t, "HTTPID", ToCamelCaseWithInitialisms("httpId"))
	require.Equal(t, "HTTPID", ToCamelCaseWithInitialism("httpId"))
}

func TestSetDefault(t *testing.T) {
	previous := Default()
	t.Cleanup(func() { SetDefault(previous) })
	generator := NewGenerator()
	SetDefault(generator)
	require.Same(t, generator, Default())
	require.PanicsWithValue(t, "codegen: nil Generator", func() { SetDefault(nil) })
	require.Same(t, generator, Default(), "rejecting nil must leave the default unchanged")
}

func TestDefaultGeneratorContentTypeHelpers(t *testing.T) {
	previous := Default()
	t.Cleanup(func() { SetDefault(previous) })
	SetDefault(NewGenerator(WithConfig(Configuration{OutputOptions: OutputOptions{
		StreamingContentTypes: []string{"^application/x-stream$"},
	}})))

	content := openapi3.Content{
		"application/x-stream": {Schema: &openapi3.SchemaRef{Value: openapi3.NewStringSchema()}},
		"text/event-stream":    {},
		"application/json":     {Schema: &openapi3.SchemaRef{Value: openapi3.NewStringSchema()}},
	}
	responses, err := GenerateResponseDefinitions("GetRecord", map[string]*openapi3.ResponseRef{
		"200": {Value: &openapi3.Response{Content: content}},
	}, "")
	require.NoError(t, err)
	require.Len(t, responses, 1)
	require.Len(t, responses[0].Contents, 3)
	for _, response := range responses[0].Contents {
		switch response.ContentType {
		case "application/x-stream":
			require.True(t, response.IsStreamingContentType())
		case "text/event-stream":
			require.True(t, response.IsStreamingContentType())
		case "application/json":
			require.False(t, response.IsStreamingContentType())
		}
	}

	bodies, _, err := GenerateBodyDefinitions("PutRecord", &openapi3.RequestBodyRef{
		Value: &openapi3.RequestBody{Content: openapi3.Content{
			"application/x-stream": content["application/x-stream"],
		}},
	}, "")
	require.NoError(t, err)
	require.Len(t, bodies, 1)
	require.True(t, (ResponseContentDefinition{ContentType: "application/x-stream"}).IsStreamingContentType())

	SetDefault(NewGenerator())
	for _, response := range responses[0].Contents {
		if response.ContentType == "application/x-stream" {
			require.True(t, response.IsStreamingContentType(), "response retains its original configuration")
		}
	}
	require.False(t, (ResponseContentDefinition{ContentType: "application/x-stream"}).IsStreamingContentType())
	require.True(t, (ResponseContentDefinition{ContentType: "text/event-stream"}).IsStreamingContentType())
}

func TestDefaultGeneratorInvalidContentTypes(t *testing.T) {
	previous := Default()
	t.Cleanup(func() { SetDefault(previous) })
	for _, tc := range []struct {
		name    string
		options OutputOptions
		message string
	}{
		{"streaming content types", OutputOptions{StreamingContentTypes: []string{"["}}, "invalid streaming-content-type pattern"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			SetDefault(NewGenerator(WithConfig(Configuration{OutputOptions: tc.options})))
			_, err := GenerateResponseDefinitions("GetRecord", map[string]*openapi3.ResponseRef{
				"200": {Value: &openapi3.Response{Content: openapi3.Content{
					"application/json": {Schema: &openapi3.SchemaRef{Value: openapi3.NewStringSchema()}},
				}}},
			}, "")
			require.ErrorContains(t, err, tc.message)
			if len(tc.options.StreamingContentTypes) > 0 {
				require.Panics(t, func() {
					(ResponseContentDefinition{ContentType: "text/event-stream"}).IsStreamingContentType()
				})
			}
		})
	}
}

func TestSetDefaultConcurrent(t *testing.T) {
	previous := Default()
	t.Cleanup(func() { SetDefault(previous) })
	first := NewGenerator(WithConfig(Configuration{OutputOptions: OutputOptions{ResponseTypeSuffix: "First"}}))
	second := NewGenerator(WithConfig(Configuration{OutputOptions: OutputOptions{ResponseTypeSuffix: "Second"}}))
	SetDefault(first)
	run := newDefaultState()

	var workers sync.WaitGroup
	for i := range 8 {
		workers.Go(func() {
			for range 20 {
				generator := first
				if i%2 == 0 {
					generator = second
				}
				SetDefault(generator)
				if got := Default(); got != first && got != second {
					t.Errorf("unexpected default generator: %p", got)
				}
				if got := genResponseTypeName("Record"); got != "RecordFirst" && got != "RecordSecond" {
					t.Errorf("unexpected response type name: %s", got)
				}
			}
		})
	}
	workers.Wait()
	SetDefault(second)
	require.Equal(t, "RecordFirst", run.genResponseTypeName("Record"))
	require.Equal(t, "RecordSecond", genResponseTypeName("Record"))
}
