package test

import "os"

func LoadFixture(relativePath string) ([]byte, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	return os.ReadFile(wd + string(os.PathSeparator) + relativePath)
}
