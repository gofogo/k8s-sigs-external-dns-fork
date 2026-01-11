# FQDN Template TypeMeta Optimization

## Problem

In `source/fqdn/fqdn.go`, the `ExecTemplate` function populates `Kind` and `APIVersion` on Kubernetes objects before executing templates. This is needed because Kubernetes informers don't populate TypeMeta.

Currently, this check runs unconditionally for every object, but it's only needed when the template actually uses `.Kind` or `.APIVersion`.

The naive fix:

```go
tmplText := tmpl.Tree.Root.String()
needsTypeMeta := strings.Contains(tmplText, ".Kind") || strings.Contains(tmplText, ".APIVersion")
```

**Issue**: `tmpl.Tree.Root.String()` reconstructs the template string by walking the parse tree on every call - inefficient when called for every Kubernetes object.

## Options

### Option 1: Wrapper Type (Recommended)

Create a wrapper type that stores the pre-computed flag:

```go
type Template struct {
    *template.Template
    needsTypeMeta bool
}

func ParseTemplate(input string) (*Template, error) {
    // ...
    needsTypeMeta := strings.Contains(input, ".Kind") || strings.Contains(input, ".APIVersion")
    return &Template{Template: tmpl, needsTypeMeta: needsTypeMeta}, nil
}
```

**Pros**: Clean, no global state, check done once at parse time
**Cons**: Requires updating all callers of `ParseTemplate` and `ExecTemplate`

### Option 2: Global Cache Map

Use `sync.Map` to cache the result per template pointer:

```go
var templateNeedsTypeMeta sync.Map // map[*template.Template]bool

func ExecTemplate(tmpl *template.Template, obj kubeObject) ([]string, error) {
    needsTypeMeta, ok := templateNeedsTypeMeta.Load(tmpl)
    if !ok {
        tmplText := tmpl.Tree.Root.String()
        needsTypeMeta = strings.Contains(tmplText, ".Kind") || strings.Contains(tmplText, ".APIVersion")
        templateNeedsTypeMeta.Store(tmpl, needsTypeMeta)
    }
    // ...
}
```

**Pros**: No API changes required
**Cons**: Global state, potential memory leak if templates are created/discarded frequently

### Option 3: Check in ParseTemplate, Return Tuple

Return both template and flag without a wrapper:

```go
func ParseTemplate(input string) (*template.Template, bool, error)
```

**Pros**: Simple
**Cons**: Awkward API, requires updating all callers

## Current State

The optimization is not yet implemented. The code still unconditionally checks and populates TypeMeta for all objects.
