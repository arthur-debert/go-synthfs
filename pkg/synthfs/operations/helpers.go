package operations

import (
	"fmt"
	"io/fs"
)

// Helper functions to safely access filesystem methods through type assertions

func fsysStat(fsys interface{}, name string) (fs.FileInfo, error) {
	type statFS interface {
		Stat(name string) (fs.FileInfo, error)
	}
	if sfs, ok := fsys.(statFS); ok {
		return sfs.Stat(name)
	}
	return nil, fmt.Errorf("filesystem does not support Stat")
}

func fsysOpen(fsys interface{}, name string) (fs.File, error) {
	type openFS interface {
		Open(name string) (fs.File, error)
	}
	if ofs, ok := fsys.(openFS); ok {
		return ofs.Open(name)
	}
	return nil, fmt.Errorf("filesystem does not support Open")
}

func fsysWriteFile(fsys interface{}, name string, data []byte, perm fs.FileMode) error {
	type writeFS interface {
		WriteFile(name string, data []byte, perm fs.FileMode) error
	}
	if wfs, ok := fsys.(writeFS); ok {
		return wfs.WriteFile(name, data, perm)
	}
	return fmt.Errorf("filesystem does not support WriteFile")
}

func fsysMkdirAll(fsys interface{}, path string, perm fs.FileMode) error {
	type mkdirFS interface {
		MkdirAll(path string, perm fs.FileMode) error
	}
	if mfs, ok := fsys.(mkdirFS); ok {
		return mfs.MkdirAll(path, perm)
	}
	return fmt.Errorf("filesystem does not support MkdirAll")
}

func fsysRemove(fsys interface{}, name string) error {
	type removeFS interface {
		Remove(name string) error
	}
	if rfs, ok := fsys.(removeFS); ok {
		return rfs.Remove(name)
	}
	return fmt.Errorf("filesystem does not support Remove")
}

func fsysRemoveAll(fsys interface{}, path string) error {
	type removeFS interface {
		RemoveAll(path string) error
	}
	if rfs, ok := fsys.(removeFS); ok {
		return rfs.RemoveAll(path)
	}
	return fmt.Errorf("filesystem does not support RemoveAll")
}

func fsysRename(fsys interface{}, oldpath, newpath string) error {
	type renameFS interface {
		Rename(oldpath, newpath string) error
	}
	if rnfs, ok := fsys.(renameFS); ok {
		return rnfs.Rename(oldpath, newpath)
	}
	return fmt.Errorf("filesystem does not support Rename")
}

func fsysSymlink(fsys interface{}, oldname, newname string) error {
	type symlinkFS interface {
		Symlink(oldname, newname string) error
	}
	if sfs, ok := fsys.(symlinkFS); ok {
		return sfs.Symlink(oldname, newname)
	}
	return fmt.Errorf("filesystem does not support Symlink")
}