package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/bregydoc/gtranslate"
	"github.com/schollz/progressbar/v3"
)

const version = "1.0.0"

var (
	fastMode    bool
	rewriteMode bool
	sourceLang  string
	domain      string
	addLang     string
	showHelp    bool
	showVer     bool
	interrupted bool
)

func init() {
	flag.BoolVar(&fastMode, "fast", false, "Use 0.1 second delay between translations (default: 1 second)")
	flag.BoolVar(&rewriteMode, "rewrite", false, "Rewrite entire PO file from POT, keeping existing translations but removing obsolete entries")
	flag.StringVar(&sourceLang, "source-lang", "", "Source language code (required if not in POT metadata)")
	flag.StringVar(&domain, "domain", "default", "Translation domain name (default: \"default\")")
	flag.StringVar(&addLang, "add-lang", "", "Create a new PO file for the specified language (2-letter code) from POT and translate it")
	flag.BoolVar(&showHelp, "help", false, "Display usage information")
	flag.BoolVar(&showVer, "version", false, "Display version information")
}

func main() {
	flag.Parse()

	if showVer {
		fmt.Printf("potranslate version %s\n", version)
		os.Exit(0)
	}

	if showHelp {
		printHelp()
		os.Exit(0)
	}

	args := flag.Args()
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Error: Please provide a directory path\n\n")
		printHelp()
		os.Exit(1)
	}

	directory := args[0]

	// Verify directory exists
	if info, err := os.Stat(directory); err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: '%s' is not a valid directory\n", directory)
		os.Exit(1)
	}

	// Setup signal handling for Ctrl-C
	setupSignalHandler()

	// Get translation delay
	delay := time.Second
	if fastMode {
		delay = 100 * time.Millisecond
	}

	// Find POT file
	potFile := filepath.Join(directory, domain+".pot")
	if _, err := os.Stat(potFile); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: POT file '%s' not found\n", potFile)
		os.Exit(1)
	}

	fmt.Printf("Processing domain: %s\n", domain)
	fmt.Printf("POT file: %s\n", potFile)

	// Parse POT file and get source language
	potEntries, detectedSourceLang, err := parsePotFile(potFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing POT file: %v\n", err)
		os.Exit(1)
	}

	// Determine source language
	finalSourceLang := detectedSourceLang
	if finalSourceLang == "" {
		if sourceLang == "" {
			fmt.Fprintf(os.Stderr, "Error: Source language not detected in POT file and not provided via --source-lang\n")
			os.Exit(1)
		}
		finalSourceLang = sourceLang
		// Update POT file with source language
		if err := updatePotLanguage(potFile, finalSourceLang); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not update POT file metadata: %v\n", err)
		} else {
			fmt.Printf("Updated POT file with source language: %s\n", finalSourceLang)
		}
	} else if sourceLang != "" && sourceLang != finalSourceLang {
		fmt.Printf("Warning: Using source language from POT file (%s) instead of provided flag (%s)\n", finalSourceLang, sourceLang)
	}

	fmt.Printf("Source language: %s\n", finalSourceLang)

	// Handle add-lang flag: create new language file
	if addLang != "" {
		if len(addLang) != 2 {
			fmt.Fprintf(os.Stderr, "Error: Language code must be 2 letters (e.g., 'es', 'fr', 'de')\n")
			os.Exit(1)
		}

		newPoFile := filepath.Join(directory, fmt.Sprintf("%s_%s.po", domain, addLang))

		// Check if file already exists
		if _, err := os.Stat(newPoFile); err == nil {
			fmt.Fprintf(os.Stderr, "Error: PO file '%s' already exists\n", newPoFile)
			os.Exit(1)
		}

		fmt.Printf("\nCreating new language file: %s\n", filepath.Base(newPoFile))

		// Copy POT to new PO file
		if err := copyPotToPo(potFile, newPoFile, addLang); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating PO file: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Created: %s\n", filepath.Base(newPoFile))
		fmt.Printf("Translating to: %s\n\n", addLang)

		// Translate the new file
		translated, err := translatePoFile(newPoFile, potEntries, finalSourceLang, addLang, delay)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error translating new PO file: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\nComplete! Translated %d string(s)\n", translated)
		os.Exit(0)
	}

	// Find all PO files for this domain
	poFiles, err := findPoFiles(directory, domain)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding PO files: %v\n", err)
		os.Exit(1)
	}

	if len(poFiles) == 0 {
		fmt.Printf("No PO files found for domain '%s'\n", domain)
		os.Exit(0)
	}

	fmt.Printf("Found %d PO file(s)\n\n", len(poFiles))

	// Process each PO file
	totalTranslated := 0

	for _, poFile := range poFiles {
		if interrupted {
			fmt.Println("\nInterrupted by user. Exiting...")
			break
		}

		targetLang, err := getTargetLanguage(poFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not determine target language for %s: %v\n", filepath.Base(poFile), err)
			continue
		}

		fmt.Printf("Processing: %s (target: %s)\n", filepath.Base(poFile), targetLang)

		var translated int
		if rewriteMode {
			translated, err = rewritePoFile(poFile, potEntries, finalSourceLang, targetLang, delay)
		} else {
			translated, err = translatePoFile(poFile, potEntries, finalSourceLang, targetLang, delay)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error processing %s: %v\n", filepath.Base(poFile), err)
			continue
		}

		totalTranslated += translated
		fmt.Printf("Translated %d string(s)\n\n", translated)
	}

	if interrupted {
		fmt.Printf("\nPartially completed: %d translation(s) saved\n", totalTranslated)
		os.Exit(130) // Standard exit code for SIGINT
	} else {
		fmt.Printf("Complete! Translated %d string(s) total\n", totalTranslated)
	}
}

func setupSignalHandler() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		interrupted = true
	}()
}

func printHelp() {
	fmt.Println("potranslate - Translate missing strings in PO files in a given directory")
	fmt.Printf("\nUsage: potranslate [options] <directory>\n\n")
	fmt.Println("Options:")
	flag.PrintDefaults()
	fmt.Println("\nExamples:")
	fmt.Println("  potranslate ./locales")
	fmt.Println("  potranslate --fast ./locales")
	fmt.Println("  potranslate --source-lang en ./locales")
	fmt.Println("  potranslate --domain admin ./locales")
	fmt.Println("  potranslate --rewrite ./locales")
	fmt.Println("  potranslate --fast --source-lang en --domain admin ./locales")
	fmt.Println("  potranslate --rewrite --fast ./locales")
	fmt.Println("  potranslate --add-lang de ./locales")
	fmt.Println("  potranslate --add-lang ja --fast ./locales")
}

func findPoFiles(directory, domain string) ([]string, error) {
	// Only support underscore naming: domain_*.po
	pattern := filepath.Join(directory, domain+"_*.po")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	return matches, nil
}

type POEntry struct {
	Msgstr   string
	Comments []string
}

func parsePotFile(potFile string) (map[string]POEntry, string, error) {
	file, err := os.Open(potFile)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	entries := make(map[string]POEntry)
	var currentMsgid, currentMsgstr string
	var currentComments []string
	var pendingComments []string
	var inMsgid, inMsgstr bool
	var sourceLang string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Check for Language header
		if strings.Contains(line, "\"Language:") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				// Remove quotes, \n escape sequence, and whitespace
				lang := strings.TrimSpace(parts[1])
				lang = strings.TrimPrefix(lang, "\"")
				lang = strings.TrimSuffix(lang, "\\n\"")
				lang = strings.TrimSuffix(lang, "\"")
				if lang != "" {
					sourceLang = lang
				}
			}
		}

		if strings.HasPrefix(trimmed, "msgid ") {
			// Save previous entry
			if currentMsgid != "" {
				entries[currentMsgid] = POEntry{
					Msgstr:   currentMsgstr,
					Comments: currentComments,
				}
			}
			currentMsgid = extractString(trimmed[6:])
			currentMsgstr = ""
			currentComments = pendingComments
			pendingComments = []string{}
			inMsgid = true
			inMsgstr = false
		} else if strings.HasPrefix(trimmed, "msgstr ") {
			currentMsgstr = extractString(trimmed[7:])
			inMsgid = false
			inMsgstr = true
		} else if strings.HasPrefix(trimmed, "\"") && (inMsgid || inMsgstr) {
			str := extractString(trimmed)
			if inMsgid {
				currentMsgid += str
			} else if inMsgstr {
				currentMsgstr += str
			}
		} else if strings.HasPrefix(trimmed, "#") {
			// Collect comments before next msgid
			if !inMsgid && !inMsgstr {
				pendingComments = append(pendingComments, line)
			}
			inMsgid = false
			inMsgstr = false
		} else if trimmed == "" {
			inMsgid = false
			inMsgstr = false
		}
	}

	// Save last entry
	if currentMsgid != "" {
		entries[currentMsgid] = POEntry{
			Msgstr:   currentMsgstr,
			Comments: currentComments,
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, "", err
	}

	// Remove empty msgid (header)
	delete(entries, "")

	return entries, sourceLang, nil
}

func extractString(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"") {
		s = s[1 : len(s)-1]
	}
	// Handle escape sequences
	s = strings.ReplaceAll(s, "\\n", "\n")
	s = strings.ReplaceAll(s, "\\t", "\t")
	s = strings.ReplaceAll(s, "\\\"", "\"")
	return s
}

func escapeString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}

func getTargetLanguage(poFile string) (string, error) {
	file, err := os.Open(poFile)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "\"Language:") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				// Remove quotes, \n escape sequence, and whitespace
				lang := strings.TrimSpace(parts[1])
				lang = strings.TrimPrefix(lang, "\"")
				lang = strings.TrimSuffix(lang, "\\n\"")
				lang = strings.TrimSuffix(lang, "\"")
				if lang != "" {
					return lang, nil
				}
			}
		}
	}

	// Fallback: try to extract from filename (e.g., default_es.po -> es)
	base := filepath.Base(poFile)
	parts := strings.Split(base, "_")
	if len(parts) >= 2 {
		lang := strings.TrimSuffix(parts[len(parts)-1], ".po")
		if lang != "" {
			return lang, nil
		}
	}

	return "", fmt.Errorf("could not determine target language")
}

func translatePoFile(poFile string, potEntries map[string]POEntry, sourceLang, targetLang string, delay time.Duration) (int, error) {
	// Read PO file
	content, err := os.ReadFile(poFile)
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(content), "\n")

	// First pass: collect existing msgids in PO file
	existingMsgids := make(map[string]bool)
	currentMsgid := ""
	inMsgid := false
	inMsgstr := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "msgid ") {
			currentMsgid = extractString(trimmed[6:])
			inMsgid = true
			inMsgstr = false
		} else if strings.HasPrefix(trimmed, "msgstr ") {
			if currentMsgid != "" {
				existingMsgids[currentMsgid] = true
			}
			inMsgid = false
			inMsgstr = true
		} else if strings.HasPrefix(trimmed, "\"") && inMsgid {
			currentMsgid += extractString(trimmed)
		} else if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			inMsgid = false
			inMsgstr = false
		}
	}

	// Find missing entries that need to be added
	var missingMsgids []string
	for msgid := range potEntries {
		if msgid != "" && !existingMsgids[msgid] {
			missingMsgids = append(missingMsgids, msgid)
		}
	}

	// Add missing entries to the end of the file
	if len(missingMsgids) > 0 {
		// Ensure file ends with newline
		if len(lines) > 0 && lines[len(lines)-1] != "" {
			lines = append(lines, "")
		}

		for _, msgid := range missingMsgids {
			lines = append(lines, "")
			// Add comments from POT file
			if entry, exists := potEntries[msgid]; exists && len(entry.Comments) > 0 {
				for _, comment := range entry.Comments {
					lines = append(lines, comment)
				}
			} else {
				lines = append(lines, "#: (added from POT)")
			}
			if strings.Contains(msgid, "\n") {
				// Multi-line msgid
				lines = append(lines, "msgid \"\"")
				parts := strings.Split(msgid, "\n")
				for idx, part := range parts {
					if idx < len(parts)-1 {
						lines = append(lines, fmt.Sprintf("\"%s\\n\"", escapeString(part)))
					} else if part != "" {
						lines = append(lines, fmt.Sprintf("\"%s\"", escapeString(part)))
					}
				}
			} else {
				lines = append(lines, fmt.Sprintf("msgid \"%s\"", escapeString(msgid)))
			}
			lines = append(lines, "msgstr \"\"")
		}

		// Write updated content back to file
		newContent := strings.Join(lines, "\n")
		if err := os.WriteFile(poFile, []byte(newContent), 0644); err != nil {
			return 0, fmt.Errorf("failed to add missing entries: %v", err)
		}

		fmt.Printf("Added %d missing entry/entries from POT file\n", len(missingMsgids))

		// Re-read the file for translation
		content, err = os.ReadFile(poFile)
		if err != nil {
			return 0, err
		}
		lines = strings.Split(string(content), "\n")
	}

	// Second pass: find entries that need translation
	var needsTranslation []string
	currentMsgid = ""
	currentMsgstr := ""
	inMsgid = false
	inMsgstr = false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "msgid ") {
			currentMsgid = extractString(trimmed[6:])
			inMsgid = true
			inMsgstr = false
		} else if strings.HasPrefix(trimmed, "msgstr ") {
			currentMsgstr = extractString(trimmed[7:])
			inMsgid = false
			inMsgstr = true

			// Check if this entry needs translation
			if currentMsgid != "" && currentMsgstr == "" {
				if entry, exists := potEntries[currentMsgid]; exists && entry.Msgstr == "" {
					needsTranslation = append(needsTranslation, currentMsgid)
				}
			}
		} else if strings.HasPrefix(trimmed, "\"") && inMsgid {
			currentMsgid += extractString(trimmed)
		} else if strings.HasPrefix(trimmed, "\"") && inMsgstr {
			currentMsgstr += extractString(trimmed)
		} else if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			inMsgid = false
			inMsgstr = false
		}
	}

	if len(needsTranslation) == 0 {
		// Return count of missing entries that were added
		if len(missingMsgids) > 0 {
			return 0, nil
		}
		return 0, nil
	}

	// Create translation map
	translations := make(map[string]string)

	// Create progress bar
	bar := progressbar.NewOptions(len(needsTranslation),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(40),
		progressbar.OptionSetDescription(fmt.Sprintf("[cyan]%s[reset]", filepath.Base(poFile))),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}))

	// Translate each missing string
	translatedCount := 0
	for _, msgid := range needsTranslation {
		if interrupted {
			break
		}

		translated, err := gtranslate.TranslateWithParams(
			msgid,
			gtranslate.TranslationParams{
				From: sourceLang,
				To:   targetLang,
			},
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nWarning: Translation failed for '%s': %v\n", msgid, err)
			bar.Add(1)
			continue
		}

		translations[msgid] = translated
		translatedCount++
		bar.Add(1)

		// Rate limiting
		if !interrupted && translatedCount < len(needsTranslation) {
			time.Sleep(delay)
		}
	}

	fmt.Println() // New line after progress bar

	if translatedCount == 0 {
		return 0, nil
	}

	// Update PO file with translations
	var newLines []string
	currentMsgid = ""
	inMsgid = false
	inMsgstr = false
	skipNextMsgstr := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "msgid ") {
			currentMsgid = extractString(trimmed[6:])
			inMsgid = true
			inMsgstr = false
			newLines = append(newLines, line)

			// Check if we have a translation for this msgid
			if _, exists := translations[currentMsgid]; exists {
				skipNextMsgstr = true
				// Look ahead for msgstr line
				for j := i + 1; j < len(lines); j++ {
					nextTrimmed := strings.TrimSpace(lines[j])
					if strings.HasPrefix(nextTrimmed, "msgstr ") {
						// Found msgstr, we'll replace it
						break
					} else if strings.HasPrefix(nextTrimmed, "\"") && inMsgid {
						// Continuation of msgid
						currentMsgid += extractString(nextTrimmed)
						newLines = append(newLines, lines[j])
						i = j
					} else if nextTrimmed != "" && !strings.HasPrefix(nextTrimmed, "#") {
						break
					} else {
						newLines = append(newLines, lines[j])
						i = j
					}
				}
			}
		} else if strings.HasPrefix(trimmed, "msgstr ") {
			inMsgid = false
			inMsgstr = true

			if skipNextMsgstr {
				// Replace with translation
				translation := translations[currentMsgid]
				if strings.Contains(translation, "\n") {
					// Multi-line translation
					newLines = append(newLines, "msgstr \"\"")
					parts := strings.Split(translation, "\n")
					for idx, part := range parts {
						if idx < len(parts)-1 {
							newLines = append(newLines, fmt.Sprintf("\"%s\\n\"", escapeString(part)))
						} else if part != "" {
							newLines = append(newLines, fmt.Sprintf("\"%s\"", escapeString(part)))
						}
					}
				} else {
					newLines = append(newLines, fmt.Sprintf("msgstr \"%s\"", escapeString(translation)))
				}
				skipNextMsgstr = false
			} else {
				newLines = append(newLines, line)
			}
		} else if strings.HasPrefix(trimmed, "\"") && inMsgid {
			currentMsgid += extractString(trimmed)
			newLines = append(newLines, line)
		} else {
			if !(skipNextMsgstr && strings.HasPrefix(trimmed, "\"") && i > 0 && strings.Contains(lines[i-1], "msgstr")) {
				newLines = append(newLines, line)
			}
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				inMsgid = false
				inMsgstr = false
			}
		}
	}

	// Write updated content back to file
	newContent := strings.Join(newLines, "\n")
	err = os.WriteFile(poFile, []byte(newContent), 0644)
	if err != nil {
		return 0, err
	}

	return translatedCount, nil
}

// rewritePoFile completely rewrites a PO file based on the POT file structure,
// maintaining existing translations but removing obsolete entries and their comments.
func rewritePoFile(poFile string, potEntries map[string]POEntry, sourceLang, targetLang string, delay time.Duration) (int, error) {
	// Read existing PO file to get current translations
	existingTranslations := make(map[string]string)

	content, err := os.ReadFile(poFile)
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(content), "\n")
	var currentMsgid, currentMsgstr string
	var inMsgid, inMsgstr bool
	var headerLines []string
	inHeader := true

	// Extract existing translations and header
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "msgid ") {
			if trimmed == "msgid \"\"" && inHeader {
				// This is the header entry, continue collecting it
				currentMsgid = ""
				inMsgid = true
				inMsgstr = false
			} else {
				inHeader = false
				currentMsgid = extractString(trimmed[6:])
				currentMsgstr = ""
				inMsgid = true
				inMsgstr = false
			}
		} else if strings.HasPrefix(trimmed, "msgstr ") {
			if inHeader && currentMsgid == "" {
				// Still in header
				inMsgid = false
				inMsgstr = true
			} else {
				currentMsgstr = extractString(trimmed[7:])
				inMsgid = false
				inMsgstr = true
			}
		} else if strings.HasPrefix(trimmed, "\"") {
			if inMsgid && !inHeader {
				currentMsgid += extractString(trimmed)
			} else if inMsgstr && !inHeader {
				currentMsgstr += extractString(trimmed)
			}
		} else if trimmed == "" {
			if inHeader && currentMsgid == "" {
				// End of header
				inHeader = false
			}
			if !inHeader && currentMsgid != "" {
				// Save the translation
				existingTranslations[currentMsgid] = currentMsgstr
				currentMsgid = ""
				currentMsgstr = ""
			}
			inMsgid = false
			inMsgstr = false
		} else if strings.HasPrefix(trimmed, "#") {
			if inHeader {
				headerLines = append(headerLines, line)
			}
		}

		// Collect header lines
		if inHeader {
			if currentMsgid == "" || trimmed == "" || strings.HasPrefix(trimmed, "msgid") || strings.HasPrefix(trimmed, "msgstr") || strings.HasPrefix(trimmed, "\"") {
				if !strings.HasPrefix(trimmed, "#") || len(headerLines) == 0 || line != headerLines[len(headerLines)-1] {
					if line != "" || len(headerLines) == 0 || headerLines[len(headerLines)-1] != "" {
						headerLines = append(headerLines, line)
					}
				}
			}
		}
	}

	// Save last translation if exists
	if currentMsgid != "" && !inHeader {
		existingTranslations[currentMsgid] = currentMsgstr
	}

	// Count entries that need translation
	var needsTranslation []string
	for msgid := range potEntries {
		if msgid == "" {
			continue
		}
		existingTrans, hasTranslation := existingTranslations[msgid]
		if !hasTranslation || existingTrans == "" {
			needsTranslation = append(needsTranslation, msgid)
		}
	}

	// Translate missing entries
	translations := make(map[string]string)
	translatedCount := 0

	if len(needsTranslation) > 0 {
		// Create progress bar
		bar := progressbar.NewOptions(len(needsTranslation),
			progressbar.OptionEnableColorCodes(true),
			progressbar.OptionShowCount(),
			progressbar.OptionSetWidth(40),
			progressbar.OptionSetDescription(fmt.Sprintf("[cyan]%s[reset]", filepath.Base(poFile))),
			progressbar.OptionSetTheme(progressbar.Theme{
				Saucer:        "[green]=[reset]",
				SaucerHead:    "[green]>[reset]",
				SaucerPadding: " ",
				BarStart:      "[",
				BarEnd:        "]",
			}))

		// Translate each missing string
		for _, msgid := range needsTranslation {
			if interrupted {
				break
			}

			translated, err := gtranslate.TranslateWithParams(
				msgid,
				gtranslate.TranslationParams{
					From: sourceLang,
					To:   targetLang,
				},
			)
			if err != nil {
				fmt.Fprintf(os.Stderr, "\nWarning: Translation failed for '%s': %v\n", msgid, err)
				bar.Add(1)
				continue
			}

			translations[msgid] = translated
			translatedCount++
			bar.Add(1)

			// Rate limiting
			if !interrupted && translatedCount < len(needsTranslation) {
				time.Sleep(delay)
			}
		}

		fmt.Println() // New line after progress bar
	}

	// Build new PO file from POT structure
	var newLines []string

	// Add header
	newLines = append(newLines, headerLines...)
	if len(headerLines) > 0 && headerLines[len(headerLines)-1] != "" {
		newLines = append(newLines, "")
	}

	// Add all entries from POT in order
	for msgid, potEntry := range potEntries {
		if msgid == "" {
			continue
		}

		newLines = append(newLines, "")

		// Add comments from POT (without translator comments from old PO)
		for _, comment := range potEntry.Comments {
			newLines = append(newLines, comment)
		}

		// Add msgid
		if strings.Contains(msgid, "\n") {
			// Multi-line msgid
			newLines = append(newLines, "msgid \"\"")
			parts := strings.Split(msgid, "\n")
			for idx, part := range parts {
				if idx < len(parts)-1 {
					newLines = append(newLines, fmt.Sprintf("\"%s\\n\"", escapeString(part)))
				} else if part != "" {
					newLines = append(newLines, fmt.Sprintf("\"%s\"", escapeString(part)))
				}
			}
		} else {
			newLines = append(newLines, fmt.Sprintf("msgid \"%s\"", escapeString(msgid)))
		}

		// Add msgstr (from existing translation, new translation, or empty)
		var msgstr string
		if trans, exists := translations[msgid]; exists {
			msgstr = trans
		} else if existingTrans, exists := existingTranslations[msgid]; exists {
			msgstr = existingTrans
		}

		if strings.Contains(msgstr, "\n") {
			// Multi-line msgstr
			newLines = append(newLines, "msgstr \"\"")
			parts := strings.Split(msgstr, "\n")
			for idx, part := range parts {
				if idx < len(parts)-1 {
					newLines = append(newLines, fmt.Sprintf("\"%s\\n\"", escapeString(part)))
				} else if part != "" {
					newLines = append(newLines, fmt.Sprintf("\"%s\"", escapeString(part)))
				}
			}
		} else {
			newLines = append(newLines, fmt.Sprintf("msgstr \"%s\"", escapeString(msgstr)))
		}
	}

	// Write the new PO file
	newContent := strings.Join(newLines, "\n")
	if err := os.WriteFile(poFile, []byte(newContent), 0644); err != nil {
		return 0, fmt.Errorf("failed to write rewritten PO file: %v", err)
	}

	// Count removed entries
	removedCount := 0
	for msgid := range existingTranslations {
		if _, exists := potEntries[msgid]; !exists && msgid != "" {
			removedCount++
		}
	}

	if removedCount > 0 {
		fmt.Printf("Removed %d obsolete entry/entries\n", removedCount)
	}

	return translatedCount, nil
}

func updatePotLanguage(potFile, language string) error {
	content, err := os.ReadFile(potFile)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	updated := false

	for i, line := range lines {
		if strings.Contains(line, "\"Language:") {
			// Update existing Language header
			lines[i] = fmt.Sprintf("\"Language: %s\\n\"", language)
			updated = true
			break
		}
	}

	if !updated {
		// Add Language header after Content-Type if not found
		for i, line := range lines {
			if strings.Contains(line, "\"Content-Type:") {
				// Insert after Content-Type line
				newLines := make([]string, 0, len(lines)+1)
				newLines = append(newLines, lines[:i+1]...)
				newLines = append(newLines, fmt.Sprintf("\"Language: %s\\n\"", language))
				newLines = append(newLines, lines[i+1:]...)
				lines = newLines
				updated = true
				break
			}
		}
	}

	if !updated {
		return fmt.Errorf("could not find appropriate place to insert Language header")
	}

	newContent := strings.Join(lines, "\n")
	return os.WriteFile(potFile, []byte(newContent), 0644)
}

// copyPotToPo creates a new PO file from the POT template with the specified language
func copyPotToPo(potFile, newPoFile, targetLang string) error {
	// Read POT file
	content, err := os.ReadFile(potFile)
	if err != nil {
		return fmt.Errorf("failed to read POT file: %v", err)
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string
	inHeader := true

	// Process each line
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Update Language header in the metadata section
		if inHeader && strings.Contains(line, "\"Language:") {
			newLines = append(newLines, fmt.Sprintf("\"Language: %s\\n\"", targetLang))
			continue
		}

		// Update Language-Team header if present
		if inHeader && strings.Contains(line, "\"Language-Team:") {
			newLines = append(newLines, fmt.Sprintf("\"Language-Team: %s\\n\"", strings.ToUpper(targetLang)))
			continue
		}

		// Update PO-Revision-Date with current timestamp
		if inHeader && strings.Contains(line, "\"PO-Revision-Date:") {
			currentTime := time.Now().Format("2006-01-02 15:04-0700")
			newLines = append(newLines, fmt.Sprintf("\"PO-Revision-Date: %s\\n\"", currentTime))
			continue
		}

		// Check if we're leaving the header section
		if inHeader && i > 0 && trimmed == "" && i+1 < len(lines) {
			nextTrimmed := strings.TrimSpace(lines[i+1])
			if !strings.HasPrefix(nextTrimmed, "\"") && nextTrimmed != "" {
				inHeader = false
			}
		}

		// Add the line as-is
		newLines = append(newLines, line)
	}

	// Write to new PO file
	newContent := strings.Join(newLines, "\n")
	if err := os.WriteFile(newPoFile, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write PO file: %v", err)
	}

	return nil
}
