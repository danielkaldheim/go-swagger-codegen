package codegen

// PruneUnusedModels returns only the models that are transitively reachable
// from the API surface (operation return types and parameters).
//
// A model is kept when it is referenced directly by an operation, or when it is
// referenced (transitively) by another kept model via one of its properties.
// Everything else — models that no included API can reach — is dropped.
//
// keep lists model class names to retain unconditionally (together with their
// transitive dependencies), even when no API references them — useful for
// models used only as query filters, request builders, etc. Unknown names in
// keep are ignored.
//
// This is intended to run AFTER excludeApi/excludeModel filtering, so removing
// an API also prunes the models only that API needed, and manually excluded
// models are never re-introduced.
func PruneUnusedModels(models []ModelData, apis []ApiData, keep []string) []ModelData {
	// Set of all model class names, used to distinguish model references from
	// primitives and container types (List, Map, String, ...) when scanning
	// type declaration strings like "List<Pet>" or "Map<String, Pet>".
	known := make(map[string]bool, len(models))
	byName := make(map[string]ModelData, len(models))
	for _, m := range models {
		known[m.Classname] = true
		byName[m.Classname] = m
	}

	// Roots: models referenced directly by any operation.
	reachable := make(map[string]bool)
	queue := make([]string, 0)
	enqueue := func(typeDecl string) {
		for _, name := range modelRefsIn(typeDecl, known) {
			if !reachable[name] {
				reachable[name] = true
				queue = append(queue, name)
			}
		}
	}

	for _, api := range apis {
		for _, op := range api.Operations {
			enqueue(op.ReturnType)
			enqueue(op.ReturnBaseType)
			for _, p := range op.AllParams {
				enqueue(p.DataType)
			}
		}
	}

	// Seed additional roots requested via keep (unknown names are ignored).
	for _, name := range keep {
		if known[name] && !reachable[name] {
			reachable[name] = true
			queue = append(queue, name)
		}
	}

	// Transitive closure: walk each kept model's property references.
	for len(queue) > 0 {
		name := queue[len(queue)-1]
		queue = queue[:len(queue)-1]

		m, ok := byName[name]
		if !ok {
			continue
		}
		// Aliases and top-level list/map wrappers carry their dependency in
		// DataType rather than Vars. Without following it, a reachable alias such
		// as DeletedAt -> NullTime survives pruning while its target is removed,
		// leaving generated clients with an undefined type name.
		enqueue(m.DataType)
		for _, v := range m.Vars {
			enqueue(v.Datatype)
			enqueue(v.ComplexType)
			if v.Items != nil {
				enqueue(v.Items.Datatype)
			}
		}
	}

	// Preserve original ordering.
	filtered := make([]ModelData, 0, len(reachable))
	for _, m := range models {
		if reachable[m.Classname] {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

// modelRefsIn extracts the model class names present in a Dart type declaration.
// It tokenizes the string on any character that cannot appear in an identifier
// (so "List<Map<String, Pet>>" yields List, Map, String, Pet) and keeps only
// tokens that are known model names.
func modelRefsIn(typeDecl string, known map[string]bool) []string {
	if typeDecl == "" {
		return nil
	}
	var refs []string
	var seen map[string]bool
	start := -1
	flush := func(end int) {
		if start < 0 {
			return
		}
		token := typeDecl[start:end]
		start = -1
		if !known[token] {
			return
		}
		if seen == nil {
			seen = make(map[string]bool)
		}
		if seen[token] {
			return
		}
		seen[token] = true
		refs = append(refs, token)
	}
	for i := 0; i < len(typeDecl); i++ {
		if isIdentChar(typeDecl[i]) {
			if start < 0 {
				start = i
			}
			continue
		}
		flush(i)
	}
	flush(len(typeDecl))
	return refs
}

func isIdentChar(c byte) bool {
	return c == '_' ||
		(c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9')
}
