package allocdir

import (
	"fmt"
	"os"
	"os/user"
	"strconv"

	"golang.org/x/sys/unix"
)

var nobody user.User

func nobodyUser() user.User {
	return nobody
}

// dropDirPermissions gives full access to a directory to all users and sets
// the owner to nobody.
func dropDirPermissions(path string, desired os.FileMode) error {
	if err := os.Chmod(path, desired|0777); err != nil {
		return fmt.Errorf("Chmod(%v) failed: %v", path, err)
	}

	// Can't change owner if not root.
	if unix.Geteuid() != 0 {
		return nil
	}

	nobody := nobodyUser()

	uid, err := getUid(&nobody)
	if err != nil {
		return err
	}

	gid, err := getGid(&nobody)
	if err != nil {
		return err
	}

	if err := os.Chown(path, uid, gid); err != nil {
		return fmt.Errorf("Couldn't change owner/group of %v to (uid: %v, gid: %v): %v", path, uid, gid, err)
	}

	return nil
}

// getUid for a user
func getUid(u *user.User) (int, error) {
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return 0, fmt.Errorf("Unable to convert Uid to an int: %v", err)
	}

	return uid, nil
}

// getGid for a user
func getGid(u *user.User) (int, error) {
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return 0, fmt.Errorf("Unable to convert Gid to an int: %v", err)
	}

	return gid, nil
}
