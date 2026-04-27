// Code generator: reads spec.json and emits Cobra CLI files under cmd/.
// Run from the project root: go run ./gen_cli
package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"regexp"
	"sort"
	"strings"
)

var (
	reField     = regexp.MustCompile(`[-_.]`)
	reSeg       = regexp.MustCompile(`[-_]`)
	reWS        = regexp.MustCompile(`[\r\n\t]+`)
	reNewPrefix = regexp.MustCompile(`(?i)^new\s*-\s*`)
	reFlagChar  = regexp.MustCompile(`[^a-z0-9-]`)
	reSubtag    = regexp.MustCompile(`[^a-z0-9]+`)
)

func capitalize(s string) string {
	// capitalize a string.
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// camelCase transforms a string unto camel case format.
// e.g. test-test -> TestTest
func camelCase(name string) string {
	var b strings.Builder
	for _, p := range reField.Split(name, -1) {
		if p != "" {
			b.WriteString(capitalize(p))
		}
	}
	return b.String()
}

// segmentToCamelCase converts a path segment (dash/underscore separated) to TitleCase.
func segmentToCamelCase(seg string) string {
	seg = strings.TrimLeft(seg, ".")
	var b strings.Builder
	for _, p := range reSeg.Split(seg, -1) {
		if p != "" {
			b.WriteString(capitalize(p))
		}
	}
	return b.String()
}

// deriveClientMethod reconstructs the oapi-codegen method name for a path+method pair.
func deriveClientMethod(httpMethod, path string) string {
	var name strings.Builder
	name.WriteString(capitalize(strings.ToLower(httpMethod)))

	for seg := range strings.SplitSeq(strings.Trim(path, "/"), "/") {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			for _, p := range reSeg.Split(seg[1:len(seg)-1], -1) {
				if p != "" {
					name.WriteString(capitalize(p))
				}
			}
		} else if seg != "" {
			name.WriteString(segmentToCamelCase(seg))
		}
	}
	return name.String()
}

// cmdUse builds the Cobra Use string, stripping the group prefix and normalising underscores.
func cmdUse(httpMethod, path, group string) string {
	segs := strings.Split(strings.Trim(path, "/"), "/")
	start := 0
	if len(segs) > 0 && strings.EqualFold(segs[0], group) {
		start = 1
	}
	parts := []string{strings.ToLower(httpMethod)}
	for _, seg := range segs[start:] {
		clean := strings.ToLower(strings.TrimLeft(seg, "."))
		clean = strings.ReplaceAll(clean, "_", "-") // normalise underscores → hyphens
		if strings.HasPrefix(clean, "{") && strings.HasSuffix(clean, "}") {
			parts = append(parts, clean[1:len(clean)-1])
		} else if clean != "" {
			parts = append(parts, clean)
		}
	}
	return strings.Join(parts, "-")
}

// useToGoVar converts a Use string to a camelCase Go variable prefix.
func useToGoVar(use string) string {
	parts := strings.Split(use, "-")
	if len(parts) == 0 {
		return use
	}
	var b strings.Builder
	b.WriteString(parts[0])
	for _, p := range parts[1:] {
		if p != "" {
			b.WriteString(capitalize(p))
		}
	}
	return b.String()
}

func specGoType(schema map[string]any) string {
	if schema == nil {
		return "string"
	}
	switch t, _ := schema["type"].(string); t {
	case "integer":
		return "int"
	case "number":
		return "float64"
	case "boolean":
		return "bool"
	}
	return "string"
}

func cobraFlagFn(gt string) string {
	switch gt {
	case "int":
		return "Int"
	case "float64":
		return "Float64"
	case "bool":
		return "Bool"
	}
	return "String"
}

func zeroVal(gt string) string {
	switch gt {
	case "int", "float64":
		return "0"
	case "bool":
		return "false"
	}
	return `""`
}

func safeStr(s string) string {
	s = reWS.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	s = reNewPrefix.ReplaceAllString(s, "") // strip "New - " / "New- " etc.
	runes := []rune(s)
	if len(runes) > 100 {
		s = string(runes[:100])
	}
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

func safeFlagName(name string) string {
	return strings.Trim(reFlagChar.ReplaceAllString(strings.ToLower(name), "-"), "-")
}

type Param struct {
	name        string
	in          string
	description string
	localField  string // name in the flags struct (may have Path/Query suffix)
	schema      map[string]any
	apiField    string // name in the oapi-codegen Params struct
}

type Op struct {
	use          string
	vn           string
	summary      string
	clientMethod string
	pathParams   []Param
	queryParams  []Param
	hasBody      bool
	hasQP        bool
	paramsType   string
}

type FileKey struct{ group, subtag string }

func parseParam(m map[string]any) Param {
	p := Param{}
	p.name, _ = m["name"].(string)
	p.in, _ = m["in"].(string)
	p.description, _ = m["description"].(string)
	if s, ok := m["schema"].(map[string]any); ok {
		p.schema = s
	}
	return p
}

// resolveFields disambiguates flags-struct field names when a path param and a
// query param would both map to the same PascalCase name.
func resolveFields(pathPars, queryPars []Param) ([]Param, []Param) {
	pathFields := map[string]bool{}
	queryFields := map[string]bool{}
	for _, p := range pathPars {
		pathFields[camelCase(p.name)] = true
	}
	for _, p := range queryPars {
		queryFields[camelCase(p.name)] = true
	}

	annotate := func(pars []Param, suffix string, other map[string]bool) []Param {
		out := make([]Param, len(pars))
		for i, p := range pars {
			f := camelCase(p.name)
			p2 := p
			p2.apiField = f
			if other[f] {
				p2.localField = f + suffix
			} else {
				p2.localField = f
			}
			out[i] = p2
		}
		return out
	}

	return annotate(pathPars, "Path", queryFields),
		annotate(queryPars, "Query", pathFields)
}

func apiGroup(tags []string) string {
	if len(tags) == 0 {
		return "other"
	}
	t := strings.ToLower(tags[0])
	for _, prefix := range []string{"mam", "mdm", "mcm", "mem", "system"} {
		if strings.Contains(t, prefix) {
			return prefix
		}
	}
	return "other"
}

func fileSubtag(tags []string) string {
	var subtag string
	switch {
	case len(tags) > 1:
		subtag = tags[1]
	case len(tags) > 0:
		subtag = tags[0]
	default:
		subtag = "general"
	}
	return strings.Trim(reSubtag.ReplaceAllString(strings.ToLower(subtag), "_"), "_")
}

func generateLeafFile(group string, ops []Op) string {
	var sb strings.Builder
	l := func(s string) { sb.WriteString(s); sb.WriteByte('\n') }

	l("// Code generated by gen_cli DO NOT EDIT.")
	l("")
	l("package gen")
	l("")
	l("import (")
	l("\t\"io\"")
	l("\t\"strings\"")
	l("")
	l("\t\"github.com/ancalabrese/ws1cli/internal/cli\"")
	l("\t\"github.com/ancalabrese/ws1cli/ws1\"")
	l("\t\"github.com/spf13/cobra\"")
	l(")")
	l("")
	l("var (")
	l("\t_ = strings.NewReader")
	l("\t_ = io.Discard")
	l("\t_ = (*cobra.Command)(nil)")
	l("\t_ = (*ws1.ClientWithResponses)(nil)")
	l("\t_ = cli.RootCmd")
	l(")")
	l("")

	for _, op := range ops {
		pp, qp := resolveFields(op.pathParams, op.queryParams)
		hasFlags := len(pp) > 0 || len(qp) > 0 || op.hasBody

		if hasFlags {
			l(fmt.Sprintf("var %sFlags struct {", op.vn))
			for _, p := range pp {
				l(fmt.Sprintf("\t%s %s", p.localField, specGoType(p.schema)))
			}
			for _, p := range qp {
				l(fmt.Sprintf("\t%s %s", p.localField, specGoType(p.schema)))
			}
			if op.hasBody {
				l("\tBody string")
			}
			l("}")
			l("")
		}

		l(fmt.Sprintf("var %sCmd = &cobra.Command{", op.vn))
		l(fmt.Sprintf("\tUse:   \"%s\",", op.use))
		l(fmt.Sprintf("\tShort: \"%s\",", op.summary))
		l(fmt.Sprintf("\tRunE:  run%s,", capitalize(op.vn)))
		l("}")
		l("")

		l("func init() {")
		l(fmt.Sprintf("\t%sCmd.AddCommand(%sCmd)", group, op.vn))
		for _, p := range pp {
			flag := safeFlagName(p.name)
			gt := specGoType(p.schema)
			desc := safeStr(p.description)
			if desc == "" {
				desc = p.name
			}
			l(fmt.Sprintf("\t%sCmd.Flags().%sVar(&%sFlags.%s, \"%s\", %s, \"%s (required)\")",
				op.vn, cobraFlagFn(gt), op.vn, p.localField, flag, zeroVal(gt), desc))
			l(fmt.Sprintf("\t_ = %sCmd.MarkFlagRequired(\"%s\")", op.vn, flag))
		}
		for _, p := range qp {
			flag := safeFlagName(p.name)
			gt := specGoType(p.schema)
			desc := safeStr(p.description)
			if desc == "" {
				desc = p.name
			}
			l(fmt.Sprintf("\t%sCmd.Flags().%sVar(&%sFlags.%s, \"%s\", %s, \"%s\")",
				op.vn, cobraFlagFn(gt), op.vn, p.localField, flag, zeroVal(gt), desc))
		}
		if op.hasBody {
			l(fmt.Sprintf("\t%sCmd.Flags().StringVar(&%sFlags.Body, \"body\", \"\", \"JSON request body\")", op.vn, op.vn))
		}
		l("}")
		l("")

		l(fmt.Sprintf("func run%s(cmd *cobra.Command, args []string) error {", capitalize(op.vn)))
		l("\tclient, err := cli.NewClient()")
		l("\tif err != nil {")
		l("\t\treturn err")
		l("\t}")

		if op.hasQP {
			l(fmt.Sprintf("\tparams := &ws1.%s{}", op.paramsType))
			for _, p := range qp {
				gt := specGoType(p.schema)
				if gt == "string" {
					l(fmt.Sprintf("\tif v := %sFlags.%s; v != \"\" {", op.vn, p.localField))
					l(fmt.Sprintf("\t\tparams.%s = &v", p.apiField))
					l("\t}")
				} else {
					l("\t{")
					l(fmt.Sprintf("\t\tv := %sFlags.%s", op.vn, p.localField))
					l(fmt.Sprintf("\t\tparams.%s = &v", p.apiField))
					l("\t}")
				}
			}
		}

		callArgs := []string{"cmd.Context()"}
		for _, p := range pp {
			callArgs = append(callArgs, fmt.Sprintf("%sFlags.%s", op.vn, p.localField))
		}
		if op.hasQP {
			callArgs = append(callArgs, "params")
		}
		var methodCall string
		if op.hasBody {
			callArgs = append(callArgs, "\"application/json\"")
			callArgs = append(callArgs, fmt.Sprintf("strings.NewReader(%sFlags.Body)", op.vn))
			methodCall = op.clientMethod + "WithBody"
		} else {
			methodCall = op.clientMethod
		}

		l(fmt.Sprintf("\thttpResp, err := client.%s(%s)", methodCall, strings.Join(callArgs, ", ")))
		l("\tif err != nil {")
		l("\t\treturn err")
		l("\t}")
		l("\tdefer httpResp.Body.Close()")
		l("\tbody, err := io.ReadAll(httpResp.Body)")
		l("\tif err != nil {")
		l("\t\treturn err")
		l("\t}")
		l("\treturn cli.PrintJSON(httpResp.StatusCode, body)")
		l("}")
		l("")
	}

	return sb.String()
}

func generateGroupFile(group string) string {
	var sb strings.Builder
	l := func(s string) { sb.WriteString(s); sb.WriteByte('\n') }

	l("// Code generated by gen_cli DO NOT EDIT.")
	l("")
	l("package gen")
	l("")
	l("import (")
	l("\t\"github.com/spf13/cobra\"")
	l("")
	l("\t\"github.com/ancalabrese/ws1cli/internal/cli\"")
	l(")")
	l("")
	l(fmt.Sprintf("var %sCmd = &cobra.Command{", group))
	l(fmt.Sprintf("\tUse:   \"%s\",", group))
	l(fmt.Sprintf("\tShort: \"Workspace ONE UEM %s API\",", strings.ToUpper(group)))
	l("}")
	l("")
	l("func init() {")
	l(fmt.Sprintf("\tcli.RootCmd.AddCommand(%sCmd)", group))
	l("}")

	return sb.String()
}

func main() {
	data, err := os.ReadFile("spec.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading spec.json: %v\n", err)
		os.Exit(1)
	}

	var spec map[string]any
	if err := json.Unmarshal(data, &spec); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing spec.json: %v\n", err)
		os.Exit(1)
	}

	paths, _ := spec["paths"].(map[string]any)

	fileOps := make(map[FileKey][]Op)
	seenVars := make(map[string]bool)

	// Sort paths for deterministic output.
	pathKeys := make([]string, 0, len(paths))
	for k := range paths {
		pathKeys = append(pathKeys, k)
	}
	sort.Strings(pathKeys)

	for _, path := range pathKeys {
		pathItem, _ := paths[path].(map[string]any)

		// Collect path-level parameters.
		pathLvlParams := map[string]Param{}
		if pars, ok := pathItem["parameters"].([]any); ok {
			for _, pi := range pars {
				if pm, ok := pi.(map[string]any); ok {
					p := parseParam(pm)
					if p.name != "" {
						pathLvlParams[p.name] = p
					}
				}
			}
		}

		for _, method := range []string{"get", "post", "put", "delete", "patch", "head", "options"} {
			opRaw, ok := pathItem[method]
			if !ok {
				continue
			}
			op, _ := opRaw.(map[string]any)

			var tags []string
			if tagsRaw, ok := op["tags"].([]any); ok {
				for _, t := range tagsRaw {
					if s, ok := t.(string); ok {
						tags = append(tags, s)
					}
				}
			}
			if len(tags) == 0 {
				tags = []string{"other"}
			}

			group := apiGroup(tags)
			subtag := fileSubtag(tags)

			// Merge path-level + operation-level params (op-level wins).
			merged := make(map[string]Param, len(pathLvlParams))
			maps.Copy(merged, pathLvlParams)
			if pars, ok := op["parameters"].([]any); ok {
				for _, pi := range pars {
					if pm, ok := pi.(map[string]any); ok {
						p := parseParam(pm)
						if p.name != "" {
							merged[p.name] = p
						}
					}
				}
			}

			// Deduplicate by (location, Go field name); sort names first for determinism.
			mergedNames := make([]string, 0, len(merged))
			for k := range merged {
				mergedNames = append(mergedNames, k)
			}
			sort.Strings(mergedNames)

			type seenKey struct{ in, field string }
			seen := map[seenKey]bool{}
			var pathPars, queryPars []Param
			for _, name := range mergedNames {
				p := merged[name]
				k := seenKey{p.in, camelCase(p.name)}
				if seen[k] {
					continue
				}
				seen[k] = true
				switch p.in {
				case "path":
					pathPars = append(pathPars, p)
				case "query":
					queryPars = append(queryPars, p)
				}
			}

			// Path params must be in URL order (oapi-codegen positional args).
			urlOrder := map[string]int{}
			for i, seg := range strings.Split(strings.Trim(path, "/"), "/") {
				if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
					urlOrder[seg[1:len(seg)-1]] = i
				}
			}
			sort.Slice(pathPars, func(i, j int) bool {
				return urlOrder[pathPars[i].name] < urlOrder[pathPars[j].name]
			})

			hasBody := false
			if _, ok := op["requestBody"]; ok {
				hasBody = true
			}

			summary := ""
			if s, ok := op["summary"].(string); ok {
				summary = safeStr(s)
			}
			if summary == "" {
				summary = method
			}

			clientMethod := deriveClientMethod(method, path)
			use := cmdUse(method, path, group)
			vn := useToGoVar(use)

			// Ensure unique var name.
			origVn := vn
			for counter := 1; seenVars[vn]; counter++ {
				vn = fmt.Sprintf("%s%d", origVn, counter)
			}
			seenVars[vn] = true

			key := FileKey{group, subtag}
			fileOps[key] = append(fileOps[key], Op{
				use:          use,
				vn:           vn,
				summary:      summary,
				clientMethod: clientMethod,
				pathParams:   pathPars,
				queryParams:  queryPars,
				hasBody:      hasBody,
				hasQP:        len(queryPars) > 0,
				paramsType:   capitalize(clientMethod) + "Params",
			})
		}
	}

	if err := os.MkdirAll("internal/cli/gen", 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "error creating internal/cli/gen/: %v\n", err)
		os.Exit(1)
	}

	groupsSeen := map[string]bool{}
	keys := make([]FileKey, 0, len(fileOps))
	for k := range fileOps {
		keys = append(keys, k)
		groupsSeen[k.group] = true
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].group != keys[j].group {
			return keys[i].group < keys[j].group
		}
		return keys[i].subtag < keys[j].subtag
	})

	for _, key := range keys {
		ops := fileOps[key]
		fname := fmt.Sprintf("internal/cli/gen/%s_%s.go", key.group, key.subtag)
		if err := os.WriteFile(fname, []byte(generateLeafFile(key.group, ops)), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "error writing %s: %v\n", fname, err)
			os.Exit(1)
		}
		fmt.Printf("  %s  (%d ops)\n", fname, len(ops))
	}

	sortedGroups := make([]string, 0, len(groupsSeen))
	for g := range groupsSeen {
		sortedGroups = append(sortedGroups, g)
	}
	sort.Strings(sortedGroups)

	for _, group := range sortedGroups {
		fname := fmt.Sprintf("internal/cli/gen/%s.go", group)
		if err := os.WriteFile(fname, []byte(generateGroupFile(group)), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "error writing %s: %v\n", fname, err)
			os.Exit(1)
		}
		fmt.Printf("  %s\n", fname)
	}

	fmt.Println("Done.")
}
