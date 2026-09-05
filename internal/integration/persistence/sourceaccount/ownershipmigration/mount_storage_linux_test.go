//go:build linux

package ownershipmigration

import (
	"context"
	"fmt"
	"testing"
)

// These are mountinfo fixtures, not network-storage runtime acceptance.
func TestUnprovableMountStorage(t *testing.T) {
	const base = "36 25 0:42 / / rw - ext4 /dev/root rw\n"
	accounts := []AccountEvidence{
		{ProfileDirectory: "/profiles/101/7", Previous: LegacyAccount{ID: 7}},
		{ProfileDirectory: "/profiles/202/8", Previous: LegacyAccount{ID: 8}},
	}
	for _, filesystem := range []string{"nfs", "nfs4", "cifs", "smb3", "ceph", "9p", "fuse.sshfs", "fuse", "overlay", "unknown"} {
		for _, source := range []string{"server:/shared", "alias:/different-name"} {
			t.Run(filesystem+"/"+source, func(t *testing.T) {
				mount := func(id int, point, backing string) string {
					return fmt.Sprintf("%d 36 0:%d / %s rw - %s %s rw\n", id, id, point, filesystem, backing)
				}
				for _, point := range []string{"/profiles", "/profiles/101/7", "/profiles/101/7/Default", "/receipt"} {
					t.Run(point, func(t *testing.T) {
						data := []byte(base + mount(50, point, source))
						if alias, err := receiptParentAliasesProfileSubtreeFromMountInfo("/profiles", "/receipt", data); err == nil && !alias {
							t.Fatal("unprovable related storage allowed a receipt")
						}
						if point != "/receipt" {
							if err := validateProfileSubtreesFromMountInfo(context.Background(), accounts, data); err == nil {
								t.Fatal("unprovable profile storage allowed ownership evidence")
							}
						}
					})
				}
				data := []byte(base + mount(50, "/profiles/101/7", "server:/shared") + mount(51, "/profiles/202/8", source))
				if err := validateProfileSubtreesFromMountInfo(context.Background(), accounts, data); err == nil {
					t.Fatal("different device IDs and source names are not isolation proof")
				}
				data = []byte(base + mount(50, "/profiles", "server:/shared") + mount(51, "/receipt", source))
				if alias, err := receiptParentAliasesProfileSubtreeFromMountInfo("/profiles", "/receipt", data); err == nil && !alias {
					t.Fatal("independent export mounts allowed a receipt")
				}
				data = []byte(base + mount(50, "/unrelated", source))
				if alias, err := receiptParentAliasesProfileSubtreeFromMountInfo("/profiles", "/receipt", data); err != nil || alias {
					t.Fatalf("unrelated storage affected local receipt: alias=%v err=%v", alias, err)
				}
				if err := validateProfileSubtreesFromMountInfo(context.Background(), accounts, data); err != nil {
					t.Fatalf("unrelated storage affected local profiles: %v", err)
				}
			})
		}
	}
}
