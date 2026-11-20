package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Checks shared by the window and the command line.

const maxCustomSize = 10_000_000

var schemaNameRe = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)

// Validate checks the template and the schema name(s).
func (o Options) Validate() error {
	if o.Template == nil {
		return errors.New("choose a schema type")
	}
	if !schemaNameRe.MatchString(o.Schema) {
		return errors.New("schema name: use lowercase letters, digits and _ (max 63, not starting with a digit)")
	}
	if strings.HasPrefix(o.Schema, "pg_") || o.Schema == "information_schema" {
		return errors.New("that schema name is reserved by PostgreSQL")
	}
	if o.Schema == "public" && o.DropExisting {
		return errors.New("refusing to drop schema \"public\"; choose another name")
	}
	for _, name := range o.SchemaNames() {
		if len(name) > 63 {
			return fmt.Errorf("schema name too long: %q is over 63 characters", name)
		}
	}
	return nil
}
