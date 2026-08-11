package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"slices"
	"strings"
)

const expectedStyleSchemaVersion = 2

type styleConfigurationDocument struct {
	SchemaVersion             int                                     `json:"schemaVersion"`
	Profile                   string                                  `json:"profile"`
	SourceRoots               []string                                `json:"sourceRoots"`
	GeneratedRoots            []string                                `json:"generatedRoots"`
	BuildSelections           []styleBuildSelectionDocument           `json:"buildSelections"`
	Rules                     map[string]json.RawMessage              `json:"rules"`
	PublicRoutes              []stylePublicRouteDocument              `json:"publicRoutes"`
	AllowedBoundaryFiles      []string                                `json:"allowedBoundaryFiles"`
	PackageFunctionExceptions []stylePackageFunctionExceptionDocument `json:"packageFunctionExceptions"`
	PackageVariableExceptions []stylePackageVariableExceptionDocument `json:"packageVariableExceptions"`
}

type styleBuildSelectionDocument struct {
	Name        string   `json:"name"`
	SourceRoots []string `json:"sourceRoots"`
	GOOS        string   `json:"goos"`
	GOARCH      string   `json:"goarch"`
	CGOEnabled  *bool    `json:"cgoEnabled"`
	Tags        []string `json:"tags"`
}

type stylePublicRouteDocument struct {
	Package  string `json:"package"`
	Receiver string `json:"receiver"`
	Method   string `json:"method"`
	Reason   string `json:"reason"`
	Issue    string `json:"issue"`
}

type stylePackageFunctionExceptionDocument struct {
	Glob             string `json:"glob"`
	Symbol           string `json:"symbol,omitempty"`
	SymbolPattern    string `json:"symbolPattern,omitempty"`
	ContributionKind string `json:"contributionKind,omitempty"`
	Maximum          int    `json:"maximum,omitempty"`
	Reason           string `json:"reason"`
}

type stylePackageVariableExceptionDocument struct {
	Glob   string `json:"glob"`
	Symbol string `json:"symbol"`
	Type   string `json:"type"`
	Reason string `json:"reason"`
	Issue  string `json:"issue"`
}

type styleRuleContract struct {
	Name        string
	Phase       string
	Diagnostics []string
}

func validateStyleConfigurationContract(content []byte) error {
	configuration, err := decodeStyleConfiguration(content)
	if err != nil {
		return err
	}
	if err = validateStyleConfiguration(configuration); err != nil {
		return err
	}
	if err = validateStyleRuleTable(content); err != nil {
		return err
	}
	return validateStyleDiagnosticTable(content)
}

func decodeStyleConfiguration(content []byte) (styleConfigurationDocument, error) {
	block, err := styleConfigurationJSON(content)
	if err != nil {
		return styleConfigurationDocument{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(block))
	decoder.DisallowUnknownFields()
	var configuration styleConfigurationDocument
	if err = decoder.Decode(&configuration); err != nil {
		return styleConfigurationDocument{}, fmt.Errorf("spice.style.configuration.schema: decode canonical style configuration: %w", err)
	}
	if err = requireStyleJSONEOF(decoder); err != nil {
		return styleConfigurationDocument{}, fmt.Errorf("spice.style.configuration.schema: decode canonical style configuration: %w", err)
	}
	return configuration, nil
}

func styleConfigurationJSON(content []byte) ([]byte, error) {
	const section = "## 51. Style configuration"
	_, remainder, found := bytes.Cut(content, []byte(section))
	if !found {
		return nil, errors.New("spice.style.configuration.schema: canonical style configuration section is missing")
	}
	if before, _, nextFound := bytes.Cut(remainder, []byte("\n## 52.")); nextFound {
		remainder = before
	}
	const fence = "```json\n"
	_, remainder, found = bytes.Cut(remainder, []byte(fence))
	if !found {
		return nil, errors.New("spice.style.configuration.schema: canonical JSON configuration fence is missing")
	}
	block, suffix, found := bytes.Cut(remainder, []byte("\n```"))
	if !found {
		return nil, errors.New("spice.style.configuration.schema: canonical JSON configuration fence is unterminated")
	}
	if bytes.Contains(suffix, []byte(fence)) {
		return nil, errors.New("spice.style.configuration.schema: style configuration section contains multiple JSON contracts")
	}
	return append([]byte(nil), block...), nil
}

func requireStyleJSONEOF(decoder *json.Decoder) error {
	var extra json.RawMessage
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return err
	}
	return errors.New("configuration contains more than one JSON value")
}

func validateStyleConfiguration(configuration styleConfigurationDocument) error {
	if err := validateStyleConfigurationIdentity(configuration); err != nil {
		return err
	}
	if err := validateStyleRoots("sourceRoots", configuration.SourceRoots); err != nil {
		return err
	}
	if err := validateStyleRoots("generatedRoots", configuration.GeneratedRoots); err != nil {
		return err
	}
	if err := validateStyleRules(configuration.Rules); err != nil {
		return err
	}
	if err := validateStyleBuildSelections(configuration); err != nil {
		return err
	}
	return validateStyleExceptions(configuration)
}

func validateStyleConfigurationIdentity(configuration styleConfigurationDocument) error {
	if configuration.SchemaVersion != expectedStyleSchemaVersion {
		return fmt.Errorf(
			"spice.style.configuration.schema: schemaVersion is %d, want %d",
			configuration.SchemaVersion,
			expectedStyleSchemaVersion,
		)
	}
	if configuration.Profile != "java-structured" {
		return fmt.Errorf("spice.style.configuration.schema: profile is %q, want java-structured", configuration.Profile)
	}
	return nil
}

func validateStyleExceptions(configuration styleConfigurationDocument) error {
	if len(configuration.AllowedBoundaryFiles) == 0 {
		return errors.New("spice.style.configuration.schema: allowedBoundaryFiles must not be empty")
	}
	if len(configuration.PackageFunctionExceptions) == 0 {
		return errors.New("spice.style.configuration.schema: packageFunctionExceptions must not be empty")
	}
	for _, exception := range configuration.PackageVariableExceptions {
		if exception.Glob == "" || exception.Symbol == "" || exception.Type == "" ||
			strings.TrimSpace(exception.Reason) == "" || strings.TrimSpace(exception.Issue) == "" {
			return errors.New("spice.style.configuration.schema: package variable exceptions require glob, symbol, type, reason, and issue")
		}
	}
	for _, route := range configuration.PublicRoutes {
		if route.Package == "" || route.Receiver == "" || route.Method == "" ||
			strings.TrimSpace(route.Reason) == "" || strings.TrimSpace(route.Issue) == "" {
			return errors.New("spice.style.configuration.schema: public route exceptions require package, receiver, method, reason, and issue")
		}
	}
	return nil
}

func validateStyleRoots(field string, roots []string) error {
	if len(roots) == 0 {
		return fmt.Errorf("spice.style.configuration.source-selection: %s must not be empty", field)
	}
	if !slices.IsSorted(roots) {
		return fmt.Errorf("spice.style.configuration.source-selection: %s must be sorted", field)
	}
	previous := ""
	for _, root := range roots {
		if root == previous {
			return fmt.Errorf("spice.style.configuration.source-selection: %s contains duplicate %q", field, root)
		}
		if !validStyleRoot(root) {
			return fmt.Errorf("spice.style.configuration.source-selection: %s contains invalid root %q", field, root)
		}
		previous = root
	}
	return nil
}

func validStyleRoot(root string) bool {
	return root != "" && root != "." && path.Clean(root) == root && !path.IsAbs(root) &&
		!strings.HasPrefix(root, "../") && !strings.Contains(root, "\\")
}

func validateStyleRules(rules map[string]json.RawMessage) error {
	expected := expectedStyleRuleContracts()
	want := make(map[string]struct{}, len(expected))
	for _, contract := range expected {
		want[contract.Name] = struct{}{}
	}
	for name := range rules {
		if _, found := want[name]; !found {
			return fmt.Errorf("spice.style.configuration.unsupported-rule: rule %q is not registered", name)
		}
	}
	for _, contract := range expected {
		value, found := rules[contract.Name]
		if !found {
			return fmt.Errorf("spice.style.configuration.unsupported-rule: rule %q is missing", contract.Name)
		}
		if contract.Name == "maxTypeFileLines" {
			var maximum int
			if err := json.Unmarshal(value, &maximum); err != nil || maximum < 1 || maximum > 10_000 {
				return fmt.Errorf("spice.style.configuration.schema: maxTypeFileLines must be an integer from 1 through 10000")
			}
			continue
		}
		var level string
		if err := json.Unmarshal(value, &level); err != nil ||
			!slices.Contains([]string{"error", "off", "warning"}, level) {
			return fmt.Errorf("spice.style.configuration.schema: rule %q has invalid level", contract.Name)
		}
	}
	return nil
}

func validateStyleBuildSelections(configuration styleConfigurationDocument) error {
	if len(configuration.BuildSelections) == 0 {
		return errors.New("spice.style.configuration.build-selection: buildSelections must not be empty")
	}
	state := newStyleBuildSelectionState(configuration)
	for index, selection := range configuration.BuildSelections {
		if err := state.validate(index, selection); err != nil {
			return err
		}
	}
	for _, root := range configuration.SourceRoots {
		if _, covered := state.coveredRoots[root]; !covered {
			return fmt.Errorf("spice.style.configuration.source-selection: source root %q is not covered by a build selection", root)
		}
	}
	return nil
}

type styleBuildSelectionState struct {
	declaredRoots map[string]struct{}
	coveredRoots  map[string]struct{}
	names         map[string]struct{}
	identities    map[string]struct{}
	previousKey   string
}

func newStyleBuildSelectionState(configuration styleConfigurationDocument) *styleBuildSelectionState {
	declaredRoots := make(map[string]struct{}, len(configuration.SourceRoots))
	for _, root := range configuration.SourceRoots {
		declaredRoots[root] = struct{}{}
	}
	return &styleBuildSelectionState{
		declaredRoots: declaredRoots,
		coveredRoots:  make(map[string]struct{}, len(configuration.SourceRoots)),
		names:         make(map[string]struct{}, len(configuration.BuildSelections)),
		identities:    make(map[string]struct{}, len(configuration.BuildSelections)),
	}
}

func (state *styleBuildSelectionState) validate(index int, selection styleBuildSelectionDocument) error {
	if !validStyleSelectionName(selection.Name) {
		return fmt.Errorf("spice.style.configuration.build-selection: selection %d has invalid name %q", index, selection.Name)
	}
	if _, found := state.names[selection.Name]; found {
		return fmt.Errorf("spice.style.configuration.build-selection: duplicate selection name %q", selection.Name)
	}
	state.names[selection.Name] = struct{}{}
	if selection.CGOEnabled == nil {
		return fmt.Errorf("spice.style.configuration.build-selection: selection %q omits cgoEnabled", selection.Name)
	}
	if !supportedStyleBuildPair(selection.GOOS, selection.GOARCH) {
		return fmt.Errorf(
			"spice.style.configuration.build-selection: selection %q uses unsupported pair %s/%s",
			selection.Name,
			selection.GOOS,
			selection.GOARCH,
		)
	}
	if err := validateStyleRoots("selection "+selection.Name+" sourceRoots", selection.SourceRoots); err != nil {
		return err
	}
	for _, root := range selection.SourceRoots {
		if _, found := state.declaredRoots[root]; !found {
			return fmt.Errorf(
				"spice.style.configuration.source-selection: selection %q uses undeclared root %q",
				selection.Name,
				root,
			)
		}
		state.coveredRoots[root] = struct{}{}
	}
	if err := validateStyleTags(selection.Name, selection.Tags); err != nil {
		return err
	}
	identity := styleBuildIdentity(selection)
	if _, found := state.identities[identity]; found {
		return fmt.Errorf("spice.style.configuration.build-selection: selection %q duplicates a build context", selection.Name)
	}
	state.identities[identity] = struct{}{}
	key := styleBuildSelectionKey(selection)
	if state.previousKey != "" && key < state.previousKey {
		return fmt.Errorf("spice.style.configuration.build-selection: selection %q is out of canonical order", selection.Name)
	}
	state.previousKey = key
	return nil
}

func validStyleSelectionName(name string) bool {
	matched, err := regexp.MatchString(`^[a-z0-9]+(?:-[a-z0-9]+)*$`, name)
	return err == nil && matched
}

func validateStyleTags(selection string, tags []string) error {
	if !slices.IsSorted(tags) {
		return fmt.Errorf("spice.style.configuration.build-selection: selection %q tags must be sorted", selection)
	}
	pattern := regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.]*$`)
	previous := ""
	for _, tag := range tags {
		if !pattern.MatchString(tag) || tag == "cgo" {
			return fmt.Errorf("spice.style.configuration.build-selection: selection %q has invalid tag %q", selection, tag)
		}
		if tag == previous {
			return fmt.Errorf("spice.style.configuration.build-selection: selection %q has duplicate tag %q", selection, tag)
		}
		previous = tag
	}
	return nil
}

func styleBuildIdentity(selection styleBuildSelectionDocument) string {
	return strings.Join([]string{
		selection.GOOS,
		selection.GOARCH,
		fmt.Sprint(*selection.CGOEnabled),
		strings.Join(selection.Tags, ","),
	}, "\x00")
}

func styleBuildSelectionKey(selection styleBuildSelectionDocument) string {
	return styleBuildIdentity(selection) + "\x00" +
		strings.Join(selection.SourceRoots, ",") + "\x00" + selection.Name
}

func supportedStyleBuildPair(goos, goarch string) bool {
	pairs := strings.Fields(`
		aix/ppc64 android/386 android/amd64 android/arm android/arm64
		darwin/amd64 darwin/arm64 dragonfly/amd64 freebsd/386 freebsd/amd64
		freebsd/arm freebsd/arm64 illumos/amd64 ios/amd64 ios/arm64 js/wasm
		linux/386 linux/amd64 linux/arm linux/arm64 linux/loong64 linux/mips
		linux/mips64 linux/mips64le linux/mipsle linux/ppc64 linux/ppc64le
		linux/riscv64 linux/s390x netbsd/386 netbsd/amd64 netbsd/arm
		netbsd/arm64 openbsd/386 openbsd/amd64 openbsd/arm openbsd/arm64
		openbsd/ppc64 openbsd/riscv64 plan9/386 plan9/amd64 plan9/arm
		solaris/amd64 wasip1/wasm windows/386 windows/amd64 windows/arm64
	`)
	return slices.Contains(pairs, goos+"/"+goarch)
}

func validateStyleRuleTable(content []byte) error {
	actual, err := parseStyleRuleTable(content)
	if err != nil {
		return err
	}
	expected := expectedStyleRuleContracts()
	if !styleRuleContractsEqual(actual, expected) {
		return fmt.Errorf("spice.style.configuration.unsupported-rule: rule implementation table differs from the canonical ordered contract")
	}
	return nil
}

func parseStyleRuleTable(content []byte) ([]styleRuleContract, error) {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	inTable := false
	var result []styleRuleContract
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "### 51.1 Rule implementation ownership"):
			inTable = true
		case inTable && strings.HasPrefix(line, "---"):
			return result, nil
		case inTable && strings.HasPrefix(line, "| `"):
			cells := strings.Split(line, "|")
			if len(cells) != 5 {
				return nil, errors.New("spice.style.configuration.unsupported-rule: malformed rule implementation row")
			}
			names := backtickValues(cells[1])
			diagnostics := backtickValues(cells[3])
			if len(names) != 1 || len(diagnostics) == 0 {
				return nil, errors.New("spice.style.configuration.unsupported-rule: malformed rule implementation identity")
			}
			result = append(result, styleRuleContract{
				Name:        names[0],
				Phase:       strings.TrimSpace(cells[2]),
				Diagnostics: diagnostics,
			})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan rule implementation contract: %w", err)
	}
	return nil, errors.New("spice.style.configuration.unsupported-rule: rule implementation table is unterminated")
}

func expectedStyleRuleContracts() []styleRuleContract {
	return []styleRuleContract{
		{Name: "onePrimaryTypePerFile", Phase: "structural", Diagnostics: []string{"spice.style.file.one-primary-type"}},
		{Name: "methodsInPrimaryTypeFile", Phase: "structural", Diagnostics: []string{"spice.style.file.method-owner"}},
		{Name: "fileNameMatchesType", Phase: "structural", Diagnostics: []string{"spice.style.file.name"}},
		{Name: "packageFunctions", Phase: "structural + typed", Diagnostics: []string{"spice.style.function.package-level"}},
		{Name: "explicitConstructors", Phase: "typed", Diagnostics: []string{"spice.style.constructor.explicit", "spice.style.constructor.name", "spice.style.constructor.location"}},
		{Name: "explicitManagedScopes", Phase: "typed", Diagnostics: []string{"spice.style.bean.scope"}},
		{Name: "banInit", Phase: "structural", Diagnostics: []string{"spice.style.function.init"}},
		{Name: "banMutablePackageState", Phase: "structural", Diagnostics: []string{"spice.style.package.mutable-global"}},
		{Name: "privateManagedFields", Phase: "typed", Diagnostics: []string{"spice.style.bean.fields-private"}},
		{Name: "moduleOwnership", Phase: "typed", Diagnostics: []string{"spice.style.package.module", "spice.style.module.dependency"}},
		{Name: "routeClassification", Phase: "typed", Diagnostics: []string{"spice.style.route.classification"}},
		{Name: "contextFirst", Phase: "structural", Diagnostics: []string{"spice.style.context.first", "spice.style.context.stored"}},
		{Name: "errorLast", Phase: "structural", Diagnostics: []string{"spice.style.error.last"}},
		{Name: "maxTypeFileLines", Phase: "structural", Diagnostics: []string{"spice.style.file.lines"}},
	}
}

func styleRuleContractsEqual(left, right []styleRuleContract) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].Name != right[index].Name || left[index].Phase != right[index].Phase ||
			!slices.Equal(left[index].Diagnostics, right[index].Diagnostics) {
			return false
		}
	}
	return true
}

func validateStyleDiagnosticTable(content []byte) error {
	actual := styleDiagnosticTable(content)
	expected := expectedStyleDiagnostics()
	if !slices.Equal(actual, expected) {
		return fmt.Errorf("CODE_STYLE.md diagnostic table differs from the canonical ordered namespace")
	}
	available := make(map[string]struct{}, len(actual))
	for _, code := range actual {
		available[code] = struct{}{}
	}
	for _, rule := range expectedStyleRuleContracts() {
		for _, code := range rule.Diagnostics {
			if _, found := available[code]; !found {
				return fmt.Errorf("rule %s references missing diagnostic %s", rule.Name, code)
			}
		}
	}
	return nil
}

func styleDiagnosticTable(content []byte) []string {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	inventory := false
	var result []string
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "## 52. Diagnostic codes"):
			inventory = true
		case inventory && strings.HasPrefix(line, "## 53."):
			return result
		case inventory && strings.HasPrefix(line, "| `spice.style."):
			values := backtickValues(line)
			if len(values) > 0 {
				result = append(result, values[0])
			}
		}
	}
	return result
}

func expectedStyleDiagnostics() []string {
	return []string{
		"spice.style.configuration.schema",
		"spice.style.configuration.unsupported-rule",
		"spice.style.configuration.source-selection",
		"spice.style.configuration.build-selection",
		"spice.style.file.one-primary-type",
		"spice.style.file.name",
		"spice.style.file.method-owner",
		"spice.style.file.unrelated-declaration",
		"spice.style.file.lines",
		"spice.style.function.package-level",
		"spice.style.function.init",
		"spice.style.constructor.name",
		"spice.style.constructor.location",
		"spice.style.constructor.explicit",
		"spice.style.bean.stereotype",
		"spice.style.bean.scope",
		"spice.style.bean.interface-binding",
		"spice.style.bean.fields-private",
		"spice.style.package.mutable-global",
		"spice.style.package.module",
		"spice.style.module.dependency",
		"spice.style.annotation.import",
		"spice.style.annotation.order",
		"spice.style.route.classification",
		"spice.style.context.first",
		"spice.style.context.stored",
		"spice.style.error.last",
		"spice.style.receiver.name",
		"spice.style.type.role-name",
		"spice.style.suppression.invalid",
	}
}

func backtickValues(value string) []string {
	var result []string
	for {
		start := strings.IndexByte(value, '`')
		if start < 0 {
			return result
		}
		value = value[start+1:]
		end := strings.IndexByte(value, '`')
		if end < 0 {
			return result
		}
		result = append(result, value[:end])
		value = value[end+1:]
	}
}
