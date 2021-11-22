// When the original format disk image is mounted repeatedly
// If a device of the same volume is unmounted due to a conflict of UUID or SN,
// subsequent mounting of the device may fail

// give an example:
// 1. We have an XFS volume whose device path is `/dev/vg_test/xfs_lv`,
//    and the mount path is `/home/data1`
// 2. After that we create snapshot `/dev/vg_test/xfs_lv_snap` of `/dev/vg_test/xfs_lv`
// 3. We mount the `/dev/vg_test/xfs_lv` device again when it is not unmounted
//    `/dev/vg_test/xfs_lv_snap` The mount fails due to uuid-conflict
// While we can solve this problem by `mounting -o,nouuid xxx xxx`,
// it is not suitable for ext series volumes

// By reassigning UUID,
// this module can solve the problem of repeated mount failure caused by
// `Ext2`, EX3, Ext4, XFS, and NTFS volumes
package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/go-basic/uuid"
	"github.com/go-cmd/cmd"
	"github.com/kr/pretty"
	"regexp"
	"runtime"
	"strings"
)

var (
	ErrUnsFs     = errors.New("unsupported file system type")
	ErrUnKFs     = errors.New("identify file system errors, unknown system type")
	ErrDevUUID   = errors.New("failed to match device uuid")
	ErrGenUUID   = errors.New("failed to generate new uuid")
	ErrQueryUUID = errors.New("failed to query the device uuid. procedure")
	ErrUMount    = errors.New("device uninstallation failed. procedure")
	ErrMount     = errors.New("failed to mount the device. procedure")
)

type FileSystemType string

const (
	FsXFS_ FileSystemType = "xfs"
	FsExt2 FileSystemType = "ext2"
	FsExt3 FileSystemType = "ext3"
	FsExt4 FileSystemType = "ext4"
	FsNTFs FileSystemType = "ntfs"

	// TODO more filesystem ...
	//FsJFS  FileSystemType = "jfs"
	//FsRFS  FileSystemType = "reiserfs"
)

type Caller_ string

const (
	CMount    Caller_ = "mount"
	CUMount   Caller_ = "umount"
	CNTFs3g   Caller_ = "ntfs-3g"
	CTune2FS  Caller_ = "tune2fs"
	CBlkID    Caller_ = "blkid"
	CFile     Caller_ = "file"
	CXFSAdmin Caller_ = "xfs_admin"
)

type DevMounter struct {
	args_ struct {
		dev   string
		path_ string
		ctx   interface{}
	}
	caller_ Caller_
	fs      FileSystemType
	uuid_   string
}

func NewMounterWithArgs(dev, path_ string, ctx interface{}) *DevMounter {
	d := new(DevMounter)

	d.args_.dev = dev
	d.args_.path_ = path_
	d.args_.ctx = ctx

	return d
}

func ExecCmd(cmdStr string) (r int, out string, err error) {

	//c := cmd.NewCmd("sh")
	//in := bytes.NewBuffer(nil)
	//in.WriteString(fmt.Sprintf("%s\n", cmdStr))
	//
	//s := <-c.StartWithStdin(in)
	//return s.Exit, strings.Join(s.Stdout, "\n"), s.Error

	cs := strings.Fields(cmdStr)
	c := cmd.NewCmd(cs[0], cs[1:]...)
	s := <-c.Start()
	return s.Exit, strings.Join(s.Stdout, "\n"), s.Error
}

func GetCallerByFS(fs FileSystemType) Caller_ {
	switch fs {
	case FsExt2:
		fallthrough
	case FsExt3:
		fallthrough
	case FsExt4:
		fallthrough
	case FsXFS_:
		return CMount
	case FsNTFs:
		return CNTFs3g
	default:
		panic(ErrUnsFs)
	}
}

func QueryDeviceUUID(dev string) (uuid string, err error) {
	if r, out, _ := ExecCmd(
		fmt.Sprintf("%s | grep %s", CBlkID, dev)); r != 0 {
		return "", ErrDevUUID
	} else {
		out = strings.ToLower(out)
		us := regexp.MustCompile("uuid=\"(?P<uuid>.*?)\"").FindStringSubmatch(out)
		if len(us) >= 2 {
			return us[1], nil
		}
	}
	return "", ErrDevUUID
}

func UMount(path_ string) (err error) {
	if r, _, _ := ExecCmd(
		fmt.Sprintf("%s %s", CUMount, path_)); r != 0 {
		return ErrUMount
	}
	return nil
}

func Mount(fs FileSystemType, dev, path_, ctx_ string) (err error) {

	__c := CMount
	if fs == FsNTFs {
		__c = CNTFs3g
	}

	if r, _, _ := ExecCmd(
		fmt.Sprintf("%s %s %s %s", __c, ctx_, dev, path_)); r != 0 {
		return ErrMount
	}
	return nil
}

func IsMount(path_ string) bool {
	if r, out, _ := ExecCmd(string(CMount)); r != 0 {
		panic(CMount)
	} else {
		if strings.Contains(out, fmt.Sprintf("%s ", path_)) {
			return true
		}
	}
	return false
}

func GenExtDevUUID(dev string) (err error) {
	if r, _, _ := ExecCmd(
		fmt.Sprintf("%s -U random %s", CTune2FS, dev)); r != 0 {
		return ErrGenUUID
	}
	return nil
}

func GenXFSDevUUID(uuid_ string, dev string) (err error) {
	if r, _, _ := ExecCmd(
		fmt.Sprintf("%s -U %s %s", CXFSAdmin, uuid_, dev)); r != 0 {
		return ErrGenUUID
	}
	return nil
}

func (m *DevMounter) Start() (err error) {
	if err = m.BindArgs(); err != nil {
		return err
	}
	if err = m.ChangeDevUUID(); err != nil {
		return err
	}
	if err = m.MountDevice(); err != nil {
		return err
	}
	if err = m.Check(); err != nil {
		return err
	}
	return nil
}

func (m *DevMounter) ChangeDevUUID() (err error) {
	if strings.HasPrefix(string(m.fs), "ext") {
		return m.changeEXT()
	} else if m.fs == FsNTFs {
		return m.changeNTFs()
	} else if m.fs == FsXFS_ {
		return m.changeXFS()
	}
	return ErrUnsFs
}

func (m *DevMounter) changeEXT() (err error) {

	if err = GenExtDevUUID(m.args_.dev); err != nil {
		return err
	}

	if m.uuid_, err = QueryDeviceUUID(m.args_.dev); err != nil {
		return ErrQueryUUID
	}
	return nil
}

func (m *DevMounter) changeXFS() (err error) {

	__registerXFSDev := func(fs FileSystemType, dev_, path_ string) (err_ error) {
		if err_ = Mount(m.fs, dev_, path_, "-o rw,nouuid"); err_ != nil {
			return err_
		}
		if err_ = UMount(dev_); err_ != nil {
			return err_
		}
		return nil
	}

	///////////////////////////////

	uuid_ := uuid.New()

	if err = __registerXFSDev(m.fs, m.args_.dev, m.args_.path_); err != nil {
		return err
	}
	if err := GenXFSDevUUID(uuid_, m.args_.dev); err != nil {
		return err
	}
	return nil
}

func (m *DevMounter) changeNTFs() (err error) {
	return nil // TODO change NTFs filesystem uuid ...
}

func (m *DevMounter) MountDevice() (err error) {
	return Mount(m.fs, m.args_.dev, m.args_.path_, "")
}

func (m *DevMounter) Check() (err error) {
	if IsMount(m.args_.dev) || IsMount(m.args_.path_) {
		return nil
	}
	return ErrMount
}

func (m *DevMounter) BindArgs() (err error) {
	if err = m.bindFS(); err != nil {
		return err
	}
	if err = m.bindCaller(); err != nil {
		return err
	}
	return nil
}

func (m *DevMounter) bindFS() (err error) {
	r, out, err_ := ExecCmd(fmt.Sprintf("%s -sL %s", CFile, m.args_.dev))
	out = strings.ToLower(out)

	if r != 0 {
		return err_
	}

	for _, _v := range []FileSystemType{FsExt2, FsExt3, FsExt4, FsXFS_, FsNTFs} {
		if strings.Contains(out, string(_v)) {
			m.fs = _v
			return nil
		}
	}

	return ErrUnKFs
}

func (m *DevMounter) bindCaller() (err error) {
	m.caller_ = GetCallerByFS(m.fs)
	return nil
}

func main() {
	var err error

	defer func() {
		if err != nil {
			buf := make([]byte, 2<<10)
			n := runtime.Stack(buf, false)
			pretty.Logf("failed to mount stack:\n%s", string(buf[:n]))
		}
	}()

	FDevPath := flag.String("dev", "", "device file path")
	FPath := flag.String("path", "", "mount path, an empty directory or a nonexistent path")
	FCtx := flag.String("ctx", "{}", "TODO. Reserved parameter")
	flag.Parse()

	m := NewMounterWithArgs(*FDevPath, *FPath, FCtx)
	err = m.Start()
}
