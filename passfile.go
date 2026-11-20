package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// The PostgreSQL password file (.pgpass), used by the command line (create): the
// same file and format as psql and pg_dump use. One line per connection:
//
//	host:port:database:user:password
//
// "*" matches anything, "\:" and "\\" escape a colon and a backslash, lines
// starting with # are comments. On Mac and Linux the file must be readable by
// its owner only (chmod 600), as PostgreSQL itself requires.

// passfilePath: PGPASSFILE, else the standard place of this system.
func passfilePath() string {
	if p := os.Getenv("PGPASSFILE"); p != "" {
		return p
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("APPDATA"), "postgresql", "pgpass.conf")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".pgpass")
}

type passEntry struct{ host, port, db, user, password string }

// readPassfile reads and checks the password file.
func readPassfile(path string) ([]passEntry, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("password file %s: %w", path, err)
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("password file %s can be read by others (%s): make it private with chmod 600 %s",
			path, fi.Mode().Perm(), path)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("password file %s: %w", path, err)
	}
	defer f.Close()
	var out []passEntry
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		fields := splitPassLine(line)
		if len(fields) != 5 {
			continue // not a valid line: skipped, as PostgreSQL does
		}
		out = append(out, passEntry{fields[0], fields[1], fields[2], fields[3], fields[4]})
	}
	return out, sc.Err()
}

// splitPassLine splits on ":" with "\:" and "\\" as escapes.
func splitPassLine(line string) []string {
	var fields []string
	var cur strings.Builder
	esc := false
	for _, r := range line {
		switch {
		case esc:
			cur.WriteRune(r)
			esc = false
		case r == '\\':
			esc = true
		case r == ':' && len(fields) < 4:
			fields = append(fields, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	return append(fields, cur.String())
}

// findPassword gives the password of the first matching line.
func findPassword(entries []passEntry, c ConnSettings) (string, bool) {
	port := c.Port
	if port == "" {
		port = "5432"
	}
	match := func(field, v string) bool { return field == "*" || field == v }
	for _, e := range entries {
		if match(e.host, c.Host) && match(e.port, port) && match(e.db, c.Database) && match(e.user, c.User) {
			return e.password, true
		}
	}
	return "", false
}
