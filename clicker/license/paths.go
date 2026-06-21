package license

import "path/filepath"

func KeyPaths() (privatePath, publicPath string) {
	return filepath.Join("license", "private.pem"), filepath.Join("license", "public.pem")
}
