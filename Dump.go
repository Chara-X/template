package template

import "gopkg.in/yaml.v3"

func Dump(t *Template) ([]byte, error) { return yaml.Marshal(t.tree.Root.Nodes) }
