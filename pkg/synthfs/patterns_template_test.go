package synthfs

import (
	"context"
	"strings"
	"testing"
)

func TestTemplatePatterns(t *testing.T) {
	sfs := WithIDGenerator(SequenceIDGenerator)

	t.Run("Basic template write", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		fs := NewTestFileSystemWithPaths("/workspace")

		// Simple template
		tmpl := "Hello, {{.Name}}! You have {{.Count}} messages."
		data := TemplateData{
			"Name":  "Alice",
			"Count": 5,
		}

		err := WriteTemplateFile(ctx, fs, "greeting.txt", tmpl, data)
		if err != nil {
			t.Fatalf("WriteTemplateFile failed: %v", err)
		}

		// Read and verify content
		content, err := fs.ReadFile("greeting.txt")
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}

		expected := "Hello, Alice! You have 5 messages."
		if string(content) != expected {
			t.Errorf("Expected %q, got %q", expected, string(content))
		}
	})

	t.Run("Template with complex data", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		fs := NewTestFileSystemWithPaths("/project")

		// Config file template
		tmpl := `{
	"name": "{{.Project}}",
	"version": "{{.Version}}",
	"dependencies": [{{range $i, $dep := .Dependencies}}{{if $i}}, {{end}}"{{$dep}}"{{end}}]
}`

		data := TemplateData{
			"Project":      "myapp",
			"Version":      "1.0.0",
			"Dependencies": []string{"express", "lodash", "axios"},
		}

		op := sfs.WriteTemplate("package.json", tmpl, data)
		err := op.Execute(ctx, fs)
		if err != nil {
			t.Fatalf("Template execution failed: %v", err)
		}

		// Verify it's valid JSON-like content
		content, _ := fs.ReadFile("package.json")
		contentStr := string(content)

		if !strings.Contains(contentStr, `"name": "myapp"`) {
			t.Error("Missing project name")
		}
		if !strings.Contains(contentStr, `"version": "1.0.0"`) {
			t.Error("Missing version")
		}
		if !strings.Contains(contentStr, `"express", "lodash", "axios"`) {
			t.Error("Missing dependencies")
		}
	})

	t.Run("TemplateBuilder", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		fs := NewTestFileSystemWithPaths("/app")

		// Build template with fluent API
		err := NewTemplateBuilder("config.yaml").
			WithTemplate(`
server:
  host: {{.Host}}
  port: {{.Port}}
database:
  url: {{.DbUrl}}
  pool_size: {{.PoolSize}}
`).
			Set("Host", "localhost").
			Set("Port", 8080).
			Set("DbUrl", "postgres://localhost/myapp").
			Set("PoolSize", 10).
			WithMode(0600).
			Execute(ctx, fs)

		if err != nil {
			t.Fatalf("TemplateBuilder failed: %v", err)
		}

		// Check content
		content, _ := fs.ReadFile("config.yaml")
		contentStr := string(content)

		if !strings.Contains(contentStr, "host: localhost") {
			t.Error("Missing host")
		}
		if !strings.Contains(contentStr, "port: 8080") {
			t.Error("Missing port")
		}
		if !strings.Contains(contentStr, "pool_size: 10") {
			t.Error("Missing pool size")
		}
	})

	t.Run("BatchTemplateWriter", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		fs := NewTestFileSystemWithPaths("/site")

		// Create multiple templates
		err := NewBatchTemplateWriter().
			Add("index.html", "<h1>{{.Title}}</h1><p>{{.Content}}</p>", TemplateData{
				"Title":   "Welcome",
				"Content": "Hello, world!",
			}).
			Add("about.html", "<h1>About {{.Company}}</h1>", TemplateData{
				"Company": "Acme Corp",
			}).
			AddWithMode("config.js", "export const API_URL = '{{.ApiUrl}}';", TemplateData{
				"ApiUrl": "https://api.example.com",
			}, 0755).
			Execute(ctx, fs)

		if err != nil {
			t.Fatalf("Batch template write failed: %v", err)
		}

		// Verify all files exist
		files := []string{"index.html", "about.html", "config.js"}
		for _, file := range files {
			if _, err := fs.Stat(file); err != nil {
				t.Errorf("File %q should exist", file)
			}
		}

		// Check specific content
		content, _ := fs.ReadFile("index.html")
		if !strings.Contains(string(content), "<h1>Welcome</h1>") {
			t.Error("Index template not rendered correctly")
		}
	})

	t.Run("Template validation", func(t *testing.T) {
		ctx := context.Background()
		fs := NewTestFileSystemWithPaths("/test")

		// Invalid template syntax
		op := sfs.WriteTemplate("bad.txt", "{{.Name", TemplateData{"Name": "test"})
		err := op.Validate(ctx, fs)
		if err == nil {
			t.Error("Should fail validation with invalid template syntax")
		}

		// Valid template
		op = sfs.WriteTemplate("good.txt", "{{.Name}}", TemplateData{"Name": "test"})
		err = op.Validate(ctx, fs)
		if err != nil {
			t.Errorf("Should pass validation: %v", err)
		}
	})

	t.Run("Template with functions", func(t *testing.T) {
		ResetSequenceCounter()
		ctx := context.Background()
		fs := NewTestFileSystemWithPaths("/workspace")

		// Template with built-in functions
		tmpl := `{{.Items | len}} items: {{range .Items}}{{.}}, {{end}}`
		data := TemplateData{
			"Items": []string{"apple", "banana", "cherry"},
		}

		err := WriteTemplateFile(ctx, fs, "list.txt", tmpl, data)
		if err != nil {
			t.Fatalf("Template with functions failed: %v", err)
		}

		content, _ := fs.ReadFile("list.txt")
		contentStr := string(content)

		if !strings.HasPrefix(contentStr, "3 items:") {
			t.Error("Template function not working")
		}
		if !strings.Contains(contentStr, "apple, banana, cherry,") {
			t.Error("Range not working correctly")
		}
	})
}
