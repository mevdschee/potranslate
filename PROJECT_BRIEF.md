# PO Translate - Project Brief

## Project Overview

**PO Translate** is a command-line tool written in Go that automates the
translation of missing or empty strings in PO (Portable Object) translation
files. The tool leverages Google Translate to fill in untranslated entries based
on a reference POT (PO Template) file, making it easier to maintain multilingual
software projects.

## Purpose

Translation workflows for software localization often involve POT template files
that define all translatable strings, and corresponding PO files for each target
language. Manually translating missing entries across multiple language files is
time-consuming and error-prone. This tool automates the initial translation
pass, allowing translators to focus on refining and polishing the automated
translations.

## Core Features

### 1. Automatic Translation

- Scans a directory for POT files and their corresponding PO files
- Automatically adds missing POT entries to PO files (entries present in POT but
  not in PO)
- Copies comments and metadata from POT entries when adding missing entries
- Identifies empty translation strings in PO files
- Automatically translates them using Google Translate API
- Preserves existing translations (only fills in missing/empty entries)

### 1.5. Rewrite Mode

- `--rewrite` flag enables complete PO file reconstruction from POT template
- Rebuilds entire PO file structure to match current POT file
- Maintains existing translations (preserves translated strings)
- Removes obsolete entries that no longer exist in POT file
- Strips old translator comments but keeps POT source comments
- Translates any missing entries automatically
- Useful for cleaning up outdated PO files and ensuring consistency with POT
  structure

### 1.6. Add Language Mode

- `--add-lang <code>` flag creates a new PO file for a specified language
- Creates file from POT template with naming pattern `<domain>_<lang>.po`
- Requires a 2-letter ISO 639-1 language code (e.g., `es`, `fr`, `de`)
- Prevents overwriting: exits with error if target PO file already exists
- Updates metadata headers:
  - Sets `Language:` header to the target language code
  - Updates `Language-Team:` header to uppercase language code
  - Sets `PO-Revision-Date:` to current timestamp
- Automatically translates all entries in the newly created file
- Useful for quickly bootstrapping translation files for new languages
- Exits after creating and translating the new language file

### 2. Language Detection

- Automatically determines the source language from the POT file metadata
- If source language cannot be detected, requires it as a command-line parameter
- Writes the source language to POT metadata when provided via command line
- Derives target languages from PO file metadata or filename conventions
- Uses the PO file standard language specifications

### 3. Rate Limiting

- Default delay of 1 second between translation requests to respect API limits
- `--fast` flag option to reduce delay to 0.1 seconds for faster processing
- Prevents overwhelming the translation service

### 4. Progress Tracking

- Real-time progress bar showing translation progress
- Displays current file being processed
- Shows completion percentage and estimated time remaining

### 5. Graceful Interruption

- Supports Ctrl-C to abort translation process
- Saves progress before exiting (translated strings are written to files)
- Allows resuming from where it left off

## Technical Stack

### Dependencies

1. **github.com/leonelquinteros/gotext**
   - Purpose: Parsing and reading PO and POT files
   - Handles the standard gettext file format
   - Provides structured access to translation entries

2. **github.com/Conight/go-googletrans**
   - Purpose: Google Translate API integration
   - Performs automatic translation between languages
   - Supports language detection and translation

3. **github.com/schollz/progressbar/v3**
   - Purpose: Terminal progress bar visualization
   - Shows translation progress with visual feedback
   - Provides ETA and completion statistics

### Language

- **Go (Golang)**: Chosen for its excellent CLI tool support, cross-platform
  compatibility, and strong concurrency model

## Functional Requirements

### Input

- Directory path containing POT and PO files
- POT file serves as the source/reference template
- One or more PO files for target languages

### Processing Workflow

1. **Discovery Phase**
   - Scan specified directory for `.pot` files based on domain (default:
     `default.pot`)
   - Support multiple domains through different named POT files (e.g.,
     `default.pot`, `admin.pot`)
   - Locate corresponding `.po` files in the same directory
   - Match files by naming convention using underscore separator (e.g.,
     `default.pot` → `default_es.po`, `default_fr.po` or `admin.pot` →
     `admin_es.po`, `admin_fr.po`)

2. **Analysis Phase**
   - Parse POT file to extract source strings with their comments and metadata
   - Parse each PO file to identify:
     - Missing entries (present in POT but not in PO)
     - Empty translations (entries exist but have no translation)
     - In rewrite mode: All existing translations for preservation
   - Add missing entries to PO files with their comments from POT (normal mode)
   - In rewrite mode: Rebuild entire PO file structure from POT template
   - Determine source language from POT metadata (`Language` header) or
     command-line parameter
   - If source language provided via command line, update POT metadata with this
     value
   - Determine target language from each PO file metadata

3. **Translation Phase**
   - For each PO file with missing translations:
     - Initialize progress bar
     - Iterate through untranslated entries
     - Call Google Translate API with rate limiting
     - Update PO file entry with translated text
     - Increment progress bar

4. **Output Phase**
   - Write updated translations back to PO files
   - Preserve original file formatting and comments
   - Maintain PO file metadata and headers

### Command-Line Interface

```bash
potranslate [options] <directory>
```

#### Options

- `--fast`: Use 0.1 second delay between translations (default: 1 second)
- `--rewrite`: Rewrite entire PO file from POT, keeping existing translations
  but removing obsolete entries
- `--add-lang <code>`: Create a new PO file for the specified language (2-letter
  code) from POT and translate it
- `--source-lang <lang>`: Source language code (required if not in POT metadata)
- `--domain <name>`: Translation domain name (default: "default")
- `--help`: Display usage information
- `--version`: Display version information

#### Examples

```bash
# Translate with default 1-second delay and default domain
potranslate ./locales

# Fast mode with 0.1-second delay
potranslate --fast ./locales

# Specify source language when not in POT metadata
potranslate --source-lang en ./locales

# Process specific domain (e.g., admin.pot and admin_*.po files)
potranslate --domain admin ./locales

# Rewrite PO files from POT (removes obsolete entries)
potranslate --rewrite ./locales

# Combine options
potranslate --fast --source-lang en --domain admin ./locales

# Rewrite with fast mode
potranslate --rewrite --fast --source-lang en ./locales

# Create a new Spanish translation file from POT
potranslate --add-lang es --source-lang en ./locales

# Create German translation with fast mode
potranslate --add-lang de --fast --source-lang en ./locales

# Create Italian translation for admin domain
potranslate --add-lang it --domain admin --source-lang en ./locales
```

## Implementation Considerations

### Error Handling

- Gracefully handle missing POT files
- Exit with error if source language cannot be detected and not provided via
  `--source-lang`
- Warn about PO files without corresponding POT files
- Handle translation API errors (network issues, rate limits)
- Validate PO/POT file format before processing

### Performance

- Process multiple PO files sequentially
- Translation requests are rate-limited to prevent API abuse
- Progress bar updates provide user feedback during long operations

### Signal Handling

- Capture SIGINT (Ctrl-C) signal
- Flush buffered translations to disk
- Display summary of completed work before exit

### File Safety

- Create backup files before modification (optional feature)
- Atomic file writes to prevent corruption
- Preserve file permissions and ownership

## Success Criteria

1. Successfully identifies POT and PO files in a directory
2. Correctly detects source and target languages
3. Translates only missing/empty entries (preserves existing translations)
4. Displays accurate progress information
5. Handles interruption gracefully without data loss
6. Respects rate limiting to avoid API issues
7. Produces valid PO files that maintain format standards

## Future Enhancements

- Support for multiple translation services (DeepL, Microsoft Translator, etc.)
- Parallel processing of multiple PO files
- Configurable rate limits via command-line flags
- Dry-run mode to preview changes without writing files
- Filter by specific languages or files
- Integration with CI/CD pipelines
- Translation memory to reuse previous translations
- Quality scoring and confidence metrics

## Target Audience

- Software developers managing multilingual applications
- Localization teams working with gettext-based translation workflows
- Open-source projects requiring translation automation
- DevOps engineers automating localization pipelines

## Project Timeline

- Phase 1: Core functionality (directory scanning, PO/POT parsing, basic
  translation)
- Phase 2: Progress bar integration and rate limiting
- Phase 3: Signal handling and graceful interruption
- Phase 4: Testing, documentation, and release

## License

MIT
