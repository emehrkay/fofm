package fofm

import (
	"io/fs"
	"io/ioutil"
)

type Setting func(ins *FOFM) error
type WriteFile func(filename string, data []byte, perm fs.FileMode) error

var DefaultSettings = []Setting{
	// set the default writer
	FileWriter,
}

// FileWriter sets the writer to be the deafult file writer
func FileWriter(ins *FOFM) error {
	ins.Writer = func(filename string, data []byte, perm fs.FileMode) error {
		return ioutil.WriteFile(filename, data, 0644)
	}

	return nil
}
