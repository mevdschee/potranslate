package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple quoted string",
			input:    `"Hello, World!"`,
			expected: "Hello, World!",
		},
		{
			name:     "string with newline escape",
			input:    `"Hello\nWorld"`,
			expected: "Hello\nWorld",
		},
		{
			name:     "string with tab escape",
			input:    `"Hello\tWorld"`,
			expected: "Hello\tWorld",
		},
		{
			name:     "string with escaped quote",
			input:    `"Hello \"World\""`,
			expected: `Hello "World"`,
		},
		{
			name:     "unquoted string",
			input:    "Hello",
			expected: "Hello",
		},
		{
			name:     "empty string",
			input:    `""`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractString(tt.input)
			if result != tt.expected {
				t.Errorf("extractString(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestEscapeString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple string",
			input:    "Hello, World!",
			expected: "Hello, World!",
		},
		{
			name:     "string with newline",
			input:    "Hello\nWorld",
			expected: "Hello\\nWorld",
		},
		{
			name:     "string with tab",
			input:    "Hello\tWorld",
			expected: "Hello\\tWorld",
		},
		{
			name:     "string with quote",
			input:    `Hello "World"`,
			expected: `Hello \"World\"`,
		},
		{
			name:     "string with backslash",
			input:    `C:\path\to\file`,
			expected: `C:\\path\\to\\file`,
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeString(tt.input)
			if result != tt.expected {
				t.Errorf("escapeString(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParsePotFile(t *testing.T) {
	tempDir := t.TempDir()
	potFile := filepath.Join(tempDir, "test.pot")

	content := `# Test POT file
msgid ""
msgstr ""
"Project-Id-Version: Test 1.0\n"
"Language: en\n"
"Content-Type: text/plain; charset=UTF-8\n"

#: test.py:10
msgid "Hello, World!"
msgstr ""

#: test.py:15
msgid "Goodbye"
msgstr ""

#: test.py:20
msgid "Multi-line"
" string"
msgstr ""
`

	if err := os.WriteFile(potFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test POT file: %v", err)
	}

	entries, sourceLang, err := parsePotFile(potFile)
	if err != nil {
		t.Fatalf("parsePotFile() error = %v", err)
	}

	if sourceLang != "en" {
		t.Errorf("Expected source language 'en', got %q", sourceLang)
	}

	expectedEntries := map[string]struct{}{
		"Hello, World!":     {},
		"Goodbye":           {},
		"Multi-line string": {},
	}

	if len(entries) != len(expectedEntries) {
		t.Errorf("Expected %d entries, got %d", len(expectedEntries), len(entries))
	}

	for msgid := range expectedEntries {
		if _, exists := entries[msgid]; !exists {
			t.Errorf("Expected entry %q not found in parsed entries", msgid)
		}
	}
}

func TestParsePotFileNoLanguage(t *testing.T) {
	tempDir := t.TempDir()
	potFile := filepath.Join(tempDir, "test.pot")

	content := `# Test POT file
msgid ""
msgstr ""
"Project-Id-Version: Test 1.0\n"
"Content-Type: text/plain; charset=UTF-8\n"

msgid "Hello"
msgstr ""
`

	if err := os.WriteFile(potFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test POT file: %v", err)
	}

	_, sourceLang, err := parsePotFile(potFile)
	if err != nil {
		t.Fatalf("parsePotFile() error = %v", err)
	}

	if sourceLang != "" {
		t.Errorf("Expected empty source language, got %q", sourceLang)
	}
}

func TestGetTargetLanguage(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name         string
		filename     string
		content      string
		expectedLang string
		expectError  bool
	}{
		{
			name:     "language from metadata",
			filename: "test_es.po",
			content: `msgid ""
msgstr ""
"Language: es\n"

msgid "Hello"
msgstr ""
`,
			expectedLang: "es",
			expectError:  false,
		},
		{
			name:     "language from filename with underscore",
			filename: "default_de.po",
			content: `msgid ""
msgstr ""

msgid "Hello"
msgstr ""
`,
			expectedLang: "de",
			expectError:  false,
		},
		{
			name:     "no language detectable",
			filename: "test.po",
			content: `msgid ""
msgstr ""

msgid "Hello"
msgstr ""
`,
			expectedLang: "",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			poFile := filepath.Join(tempDir, tt.filename)
			if err := os.WriteFile(poFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to create test PO file: %v", err)
			}

			lang, err := getTargetLanguage(poFile)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if lang != tt.expectedLang {
					t.Errorf("Expected language %q, got %q", tt.expectedLang, lang)
				}
			}
		})
	}
}

func TestFindPoFiles(t *testing.T) {
	tempDir := t.TempDir()

	testFiles := []string{
		"default_es.po",
		"default_fr.po",
		"default_de.po",
		"admin_es.po",
		"other.po",
		"default.pot",
	}

	for _, filename := range testFiles {
		path := filepath.Join(tempDir, filename)
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	tests := []struct {
		name          string
		domain        string
		expectedCount int
		expectedFiles []string
	}{
		{
			name:          "default domain",
			domain:        "default",
			expectedCount: 3,
			expectedFiles: []string{"default_es.po", "default_fr.po", "default_de.po"},
		},
		{
			name:          "admin domain",
			domain:        "admin",
			expectedCount: 1,
			expectedFiles: []string{"admin_es.po"},
		},
		{
			name:          "nonexistent domain",
			domain:        "nonexistent",
			expectedCount: 0,
			expectedFiles: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := findPoFiles(tempDir, tt.domain)
			if err != nil {
				t.Fatalf("findPoFiles() error = %v", err)
			}

			if len(files) != tt.expectedCount {
				t.Errorf("Expected %d files, got %d", tt.expectedCount, len(files))
			}

			for _, expectedFile := range tt.expectedFiles {
				found := false
				for _, file := range files {
					if filepath.Base(file) == expectedFile {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected file %q not found in results", expectedFile)
				}
			}
		})
	}
}

func TestUpdatePotLanguage(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		language    string
		expectError bool
	}{
		{
			name: "update existing language",
			content: `msgid ""
msgstr ""
"Language: \n"
"Content-Type: text/plain; charset=UTF-8\n"
`,
			language:    "en",
			expectError: false,
		},
		{
			name: "add language after Content-Type",
			content: `msgid ""
msgstr ""
"Content-Type: text/plain; charset=UTF-8\n"
`,
			language:    "en",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			potFile := filepath.Join(tempDir, "test.pot")

			if err := os.WriteFile(potFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to create test POT file: %v", err)
			}

			err := updatePotLanguage(potFile, tt.language)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				_, sourceLang, err := parsePotFile(potFile)
				if err != nil {
					t.Fatalf("Failed to parse updated POT file: %v", err)
				}
				if sourceLang != tt.language {
					t.Errorf("Expected language %q, got %q", tt.language, sourceLang)
				}
			}
		})
	}
}

func TestAddMissingEntriesToPO(t *testing.T) {
	tempDir := t.TempDir()

	// Create a POT file with multiple entries
	potFile := filepath.Join(tempDir, "test.pot")
	potContent := `msgid ""
msgstr ""
"Language: en\n"

msgid "Hello"
msgstr ""

msgid "World"
msgstr ""

msgid "Goodbye"
msgstr ""
`
	if err := os.WriteFile(potFile, []byte(potContent), 0644); err != nil {
		t.Fatalf("Failed to create POT file: %v", err)
	}

	// Create a PO file with only some entries
	poFile := filepath.Join(tempDir, "test-es.po")
	poContent := `msgid ""
msgstr ""
"Language: es\n"

msgid "Hello"
msgstr "Hola"
`
	if err := os.WriteFile(poFile, []byte(poContent), 0644); err != nil {
		t.Fatalf("Failed to create PO file: %v", err)
	}

	// Parse POT file
	potEntries, _, err := parsePotFile(potFile)
	if err != nil {
		t.Fatalf("Failed to parse POT file: %v", err)
	}

	// Call translatePoFile (which should add missing entries)
	// We use a very short delay and will interrupt to avoid actual translation
	_, err = translatePoFile(poFile, potEntries, "en", "es", 0)
	if err != nil {
		t.Fatalf("translatePoFile failed: %v", err)
	}

	// Read the updated PO file
	updatedContent, err := os.ReadFile(poFile)
	if err != nil {
		t.Fatalf("Failed to read updated PO file: %v", err)
	}

	updatedStr := string(updatedContent)

	// Check that missing entries were added
	if !strings.Contains(updatedStr, `msgid "World"`) {
		t.Error("Missing entry 'World' was not added to PO file")
	}
	if !strings.Contains(updatedStr, `msgid "Goodbye"`) {
		t.Error("Missing entry 'Goodbye' was not added to PO file")
	}

	// Check that existing entry is still there
	if !strings.Contains(updatedStr, `msgid "Hello"`) {
		t.Error("Existing entry 'Hello' was removed")
	}
	if !strings.Contains(updatedStr, `msgstr "Hola"`) {
		t.Error("Existing translation 'Hola' was removed")
	}
}

func TestCommentsAreCopiedFromPOT(t *testing.T) {
	tempDir := t.TempDir()

	// Create a POT file with comments
	potFile := filepath.Join(tempDir, "test.pot")
	potContent := `msgid ""
msgstr ""
"Language: en\n"

#: source.py:10
msgid "First"
msgstr ""

#: source.py:20
#: source.py:30
msgid "Second"
msgstr ""

#. Translator comment
#: source.py:40
msgid "Third"
msgstr ""
`
	if err := os.WriteFile(potFile, []byte(potContent), 0644); err != nil {
		t.Fatalf("Failed to create POT file: %v", err)
	}

	// Create a PO file missing some entries
	poFile := filepath.Join(tempDir, "test-es.po")
	poContent := `msgid ""
msgstr ""
"Language: es\n"

#: source.py:10
msgid "First"
msgstr "Primero"
`
	if err := os.WriteFile(poFile, []byte(poContent), 0644); err != nil {
		t.Fatalf("Failed to create PO file: %v", err)
	}

	// Parse POT file
	potEntries, _, err := parsePotFile(potFile)
	if err != nil {
		t.Fatalf("Failed to parse POT file: %v", err)
	}

	// Call translatePoFile to add missing entries
	_, err = translatePoFile(poFile, potEntries, "en", "es", 0)
	if err != nil {
		t.Fatalf("translatePoFile failed: %v", err)
	}

	// Read the updated PO file
	updatedContent, err := os.ReadFile(poFile)
	if err != nil {
		t.Fatalf("Failed to read updated PO file: %v", err)
	}

	updatedStr := string(updatedContent)

	// Check that comments were copied for "Second"
	if !strings.Contains(updatedStr, "#: source.py:20") {
		t.Error("Comment '#: source.py:20' was not copied from POT file")
	}
	if !strings.Contains(updatedStr, "#: source.py:30") {
		t.Error("Comment '#: source.py:30' was not copied from POT file")
	}

	// Check that comments were copied for "Third"
	if !strings.Contains(updatedStr, "#. Translator comment") {
		t.Error("Translator comment was not copied from POT file")
	}
	if !strings.Contains(updatedStr, "#: source.py:40") {
		t.Error("Comment '#: source.py:40' was not copied from POT file")
	}
}

func TestCopyPotToPo(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name       string
		potContent string
		targetLang string
		wantError  bool
	}{
		{
			name: "basic POT to PO conversion",
			potContent: `# Translation Template
msgid ""
msgstr ""
"Project-Id-Version: Test 1.0\n"
"Language: en\n"
"Language-Team: English\n"
"PO-Revision-Date: 2024-01-01 12:00+0000\n"
"Content-Type: text/plain; charset=UTF-8\n"

#: test.py:10
msgid "Hello"
msgstr ""

#: test.py:20
msgid "World"
msgstr ""
`,
			targetLang: "es",
			wantError:  false,
		},
		{
			name: "POT with multi-line strings",
			potContent: `msgid ""
msgstr ""
"Language: en\n"
"Content-Type: text/plain; charset=UTF-8\n"

msgid "This is a "
"multi-line string"
msgstr ""
`,
			targetLang: "fr",
			wantError:  false,
		},
		{
			name: "POT without language metadata",
			potContent: `msgid ""
msgstr ""
"Content-Type: text/plain; charset=UTF-8\n"

msgid "Test"
msgstr ""
`,
			targetLang: "de",
			wantError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			potFile := filepath.Join(tempDir, tt.name+".pot")
			if err := os.WriteFile(potFile, []byte(tt.potContent), 0644); err != nil {
				t.Fatalf("Failed to create POT file: %v", err)
			}

			newPoFile := filepath.Join(tempDir, tt.name+"-"+tt.targetLang+".po")

			err := copyPotToPo(potFile, newPoFile, tt.targetLang)
			if (err != nil) != tt.wantError {
				t.Errorf("copyPotToPo() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if tt.wantError {
				return
			}

			// Verify the new PO file was created
			if _, err := os.Stat(newPoFile); os.IsNotExist(err) {
				t.Error("New PO file was not created")
				return
			}

			// Read and verify content
			content, err := os.ReadFile(newPoFile)
			if err != nil {
				t.Fatalf("Failed to read new PO file: %v", err)
			}

			contentStr := string(content)

			// Check that Language header was updated
			if strings.Contains(tt.potContent, "\"Language:") {
				expectedLangHeader := fmt.Sprintf("\"Language: %s\\n\"", tt.targetLang)
				if !strings.Contains(contentStr, expectedLangHeader) {
					t.Errorf("Expected Language header %q not found in PO file", expectedLangHeader)
				}
			}

			// Check that Language-Team header was updated
			if strings.Contains(tt.potContent, "\"Language-Team:") {
				expectedTeamHeader := fmt.Sprintf("\"Language-Team: %s\\n\"", strings.ToUpper(tt.targetLang))
				if !strings.Contains(contentStr, expectedTeamHeader) {
					t.Errorf("Expected Language-Team header %q not found in PO file", expectedTeamHeader)
				}
			}

			// Check that PO-Revision-Date was updated
			if strings.Contains(tt.potContent, "\"PO-Revision-Date:") {
				if !strings.Contains(contentStr, "\"PO-Revision-Date:") {
					t.Error("PO-Revision-Date header not found in PO file")
				}
				// Verify date format is reasonable (contains year 20xx)
				if !strings.Contains(contentStr, "202") {
					t.Error("PO-Revision-Date does not appear to be current")
				}
			}

			// Check that msgid entries are preserved
			if strings.Contains(tt.potContent, `msgid "Hello"`) {
				if !strings.Contains(contentStr, `msgid "Hello"`) {
					t.Error("msgid 'Hello' was not preserved in PO file")
				}
			}

			if strings.Contains(tt.potContent, `msgid "World"`) {
				if !strings.Contains(contentStr, `msgid "World"`) {
					t.Error("msgid 'World' was not preserved in PO file")
				}
			}

			// Check that comments are preserved
			if strings.Contains(tt.potContent, "#: test.py:10") {
				if !strings.Contains(contentStr, "#: test.py:10") {
					t.Error("Comment '#: test.py:10' was not preserved in PO file")
				}
			}

			// Verify msgstr entries remain empty
			if strings.Contains(tt.potContent, `msgid "Hello"`) {
				// Find Hello's msgstr
				if !strings.Contains(contentStr, `msgid "Hello"`+"\nmsgstr \"\"") &&
					!strings.Contains(contentStr, `msgid "Hello"`+"\r\nmsgstr \"\"") {
					t.Error("msgstr for 'Hello' should be empty in new PO file")
				}
			}
		})
	}
}

func TestCopyPotToPoFileErrors(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name      string
		setupFunc func() (potFile, newPoFile string)
		wantError bool
	}{
		{
			name: "nonexistent POT file",
			setupFunc: func() (string, string) {
				return filepath.Join(tempDir, "nonexistent.pot"),
					filepath.Join(tempDir, "output.po")
			},
			wantError: true,
		},
		{
			name: "invalid output directory",
			setupFunc: func() (string, string) {
				potFile := filepath.Join(tempDir, "test.pot")
				os.WriteFile(potFile, []byte("msgid \"\"\nmsgstr \"\""), 0644)
				return potFile, "/invalid/path/output.po"
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			potFile, newPoFile := tt.setupFunc()

			err := copyPotToPo(potFile, newPoFile, "es")
			if (err != nil) != tt.wantError {
				t.Errorf("copyPotToPo() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}
