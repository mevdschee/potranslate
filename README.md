# PO Translate

A command-line tool written in Go that automatically translates missing or empty
strings in PO (Portable Object) translation files using Google Translate.

## Features

- **Automatic Translation**: Translates missing/empty entries in PO files using
  Google Translate
- **Auto-Sync**: Automatically adds missing POT entries to PO files with their
  comments
- **Rewrite Mode**: Completely rebuild PO files from POT template while
  preserving translations
- **Add Language Mode**: Create new PO files for additional languages from POT
  template and translate automatically
- **Language Detection**: Auto-detects source language from POT metadata or
  accepts via command-line
- **Progress Tracking**: Real-time progress bar with completion percentage
- **Rate Limiting**: Configurable delay between translations (1s default, 0.1s
  with `--fast`)
- **Domain Support**: Handle multiple translation domains in different POT files
- **Graceful Interruption**: Ctrl-C saves progress and exits cleanly
- **Metadata Updates**: Writes source language to POT metadata when provided

## Installation

### Prerequisites

- Go 1.22 or higher

### Build from Source

```bash
git clone https://github.com/maurits/potranslate.git
cd potranslate
go build
```

Or install directly:

```bash
go install github.com/maurits/potranslate@latest
```

## Usage

### Basic Syntax

```bash
potranslate [options] <directory>
```

### Options

- `--fast`: Use 0.1 second delay between translations (default: 1 second)
- `--rewrite`: Rewrite entire PO file from POT, keeping existing translations
  but removing obsolete entries
- `--add-lang <code>`: Create a new PO file for the specified language (2-letter
  code) from POT and translate it
- `--source-lang <lang>`: Source language code (required if not in POT metadata,
  e.g., `en`, `es`, `fr`)
- `--domain <name>`: Translation domain name (default: `"default"`)
- `--help`: Display usage information
- `--version`: Display version information

### Examples

#### Basic usage with default domain

```bash
# Looks for default.pot and default_*.po files
potranslate ./locales
```

#### Fast mode

```bash
# Translate with 0.1 second delay between requests
potranslate --fast ./locales
```

#### Specify source language

```bash
# Use when source language is not in POT metadata
potranslate --source-lang en ./locales
```

#### Process specific domain

```bash
# Process admin.pot and admin_*.po files
potranslate --domain admin ./locales
```

#### Rewrite mode (rebuild PO files)

```bash
# Completely rebuild PO files from POT template
# Keeps existing translations but removes obsolete entries
potranslate --rewrite ./locales
```

#### Add a new language

```bash
# Create a new Spanish translation file from POT and translate it
potranslate --add-lang es --source-lang en ./locales

# Create German translation with fast mode
potranslate --add-lang de --fast --source-lang en ./locales

# Create Italian translation for admin domain
potranslate --add-lang it --domain admin --source-lang en ./locales
```

#### Combine options

```bash
potranslate --fast --source-lang en --domain admin ./locales

# Rewrite with fast mode
potranslate --rewrite --fast ./locales
```

## How It Works

1. **Discovery**: Scans the directory for POT and PO files based on the domain
   name
2. **Sync**: Automatically adds any missing entries from POT to PO files
   - Copies comments (like `#: file.py:123`) from POT entries
   - Preserves all metadata and formatting
   - Reports number of entries added
   - In rewrite mode: Rebuilds entire PO file structure from POT
3. **Analysis**:
   - Parses POT file to extract source strings
   - Detects source language from metadata or uses provided value
   - Identifies empty translations in PO files
   - In rewrite mode: Extracts existing translations for preservation
4. **Translation**:
   - Translates each empty entry using Google Translate
   - Shows progress with a real-time progress bar
   - Applies rate limiting to respect API limits
5. **Update**: Writes translated strings back to PO files while preserving
   formatting
   - In rewrite mode: Removes obsolete entries no longer in POT

## File Naming Conventions

The tool expects files to follow these naming patterns:

- POT file: `<domain>.pot` (e.g., `default.pot`, `admin.pot`)
- PO files: `<domain>_<lang>.po` (underscore separator only)
  - Examples: `default_es.po`, `default_fr.po`, `admin_de.po`

## Signal Handling

Press `Ctrl-C` to interrupt the translation process. The tool will:

- Stop processing new translations
- Save all completed translations to disk
- Display a summary of work completed
- Exit with code 130 (standard SIGINT exit code)

## Language Codes

Use standard ISO 639-1 language codes:

- `en` - English
- `es` - Spanish
- `fr` - French
- `de` - German
- `it` - Italian
- `pt` - Portuguese
- `ja` - Japanese
- `zh` - Chinese
- And many more...

## Example Workflow

```bash
# Directory structure
locales/
  default.pot       # Contains all source strings
  default_es.po     # Spanish translations (some missing)
  default_fr.po     # French translations (some missing)
  admin.pot         # Admin interface strings
  admin_es.po       # Spanish admin translations

# Translate default domain
potranslate --source-lang en ./locales

# Output:
# Processing domain: default
# POT file: ./locales/default.pot
# Source language: en
# Found 2 PO file(s)
# 
# Processing: default_es.po (target: es)
# Added 3 missing entry/entries from POT file
# [████████████████████████] 25/25 (100%)
# Translated 25 string(s)
#
# Processing: default_fr.po (target: fr)
# Added 5 missing entry/entries from POT file
# [████████████████████████] 18/18 (100%)
# Translated 18 string(s)
#
# Complete! Translated 43 string(s) total
```

## Limitations

- Requires internet connection for Google Translate API
- Translation quality depends on Google Translate
- Rate limiting is recommended to avoid API throttling
- Multi-line strings are supported but may have formatting variations
- Automated translations should be reviewed by human translators

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Acknowledgments

- [gtranslate](https://github.com/bregydoc/gtranslate) - Google Translate API
  wrapper
- [progressbar](https://github.com/schollz/progressbar) - Terminal progress bar
- Built for the gettext localization ecosystem
