package armazem

import "io/fs"

// mustSub recorta uma subpasta de um FS embutido. O nome é fixo no código, então erro aqui é bug.
func mustSub(f fs.FS, dir string) fs.FS {
	s, err := fs.Sub(f, dir)
	if err != nil {
		panic(err)
	}
	return s
}
