# `newid-mount`

When the original format disk image is mounted repeatedly, If a device of the same volume is unmounted due to a conflict of `UUID` or `SN`, subsequent mounting of the device may fail

## give an example:

1. We have an `XFS` volume whose device path is `/dev/vg_test/xfs_lv`,
    and the mount path is `/home/data1`
2. After that we create snapshot `/dev/vg_test/xfs_lv_snap` of `/dev/vg_test/xfs_lv`
3. We mount the `/dev/vg_test/xfs_lv` device again when it is not unmounted `/dev/vg_test/xfs_lv_snap` The mount fails due to `uuid-conflict` While we can solve this problem by `mounting -o,nouuid xxx xxx`, it is not suitable for ext series volumes

## The supported volume formats include:

* `EXT2`
* `EXT3`
* `EXT4`
* `XFS`
* `NTFS`

## Dependent tools

* `mount`
* `umount`
* `ntfs-3g`
* `tune2fs`
* `blkid`
* `file`
* `xfs_admin`

## Usage

```
Usage of ./newid-mount:
  -ctx string
        TODO. Reserved parameter (default "{}")
  -dev string
        device file path
  -path string
```

