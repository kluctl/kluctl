# Embedded Jinja2 Renderer

This library provides an embedded Jinja2 renderer, based on https://github.com/kluctl/go-embed-python. It works
by spawning one (or multiple) Jinaj2 renderer processes which communicate with your Go application via stdin/stdout.

Whenever something needs to be rendered, it is sent to the renderer, which will then process it. The result is returned
to the caller.

Here is an example:

```go
package main

import (
	"fmt"
	"github.com/kluctl/kluctl/lib/go-jinja2"
)

func main() {
	j2, err := jinja2.NewJinja2("example", 1,
		jinja2.WithGlobal("test_var1", 1),
		jinja2.WithGlobal("test_var2", map[string]any{"test": 2}))
	if err != nil {
		panic(err)
	}
	defer j2.Close()

	template := "{{ test_var1 }}"

	s, err := j2.RenderString(template)
	if err != nil {
		panic(err)
	}

	fmt.Printf("template: %s\nresult: %s", template, s)
}
```
