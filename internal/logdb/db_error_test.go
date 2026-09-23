// Copyright 2017-2021 Lei Ni (nilei81@gmail.com) and other contributors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logdb

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lni/dragonboat/v4/config"
	"github.com/lni/dragonboat/v4/internal/fileutil"
	"github.com/lni/dragonboat/v4/internal/logdb/kv"
	"github.com/lni/dragonboat/v4/internal/vfs"
	pb "github.com/lni/dragonboat/v4/raftpb"
)

var errIterate = errors.New("iterate failed")

// failingIterKV fails every IterateValue call once failing is set.
type failingIterKV struct {
	kv.IKVStore
	failing bool
}

func (f *failingIterKV) IterateValue(fk []byte, lk []byte, inc bool,
	op func(key []byte, data []byte) (bool, error)) error {
	if f.failing {
		return errIterate
	}
	return f.IKVStore.IterateValue(fk, lk, inc, op)
}

func TestSnapshotSaveErrorIsReturned(t *testing.T) {
	fs := vfs.GetTestFS()
	dir := fs.PathJoin(RDBTestDirectory, "db-dir")
	wal := fs.PathJoin(RDBTestDirectory, "wal-db-dir")
	require.NoError(t, fileutil.MkdirAll(dir, fs))
	require.NoError(t, fileutil.MkdirAll(wal, fs))
	defer deleteTestDB(fs)
	var fkv *failingIterKV
	kvf := func(cfg config.LogDBConfig, cb kv.LogDBCallback,
		dir string, wal string, fs vfs.IFS) (kv.IKVStore, error) {
		s, err := newDefaultKVStore(cfg, cb, dir, wal, fs)
		if err != nil {
			return nil, err
		}
		fkv = &failingIterKV{IKVStore: s}
		return fkv, nil
	}
	db, err := openRDB(config.GetDefaultLogDBConfig(), nil, dir, wal, false, fs, kvf)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, db.close())
	}()
	fkv.failing = true
	ud := pb.Update{
		ShardID:   1,
		ReplicaID: 2,
		Snapshot:  pb.Snapshot{Index: 10, Term: 2},
		EntriesToSave: []pb.Entry{
			{Index: 11, Term: 2},
		},
	}
	require.ErrorIs(t, db.saveRaftState([]pb.Update{ud}, newContext(128, 128)), errIterate)
	ud.Snapshot.Index, ud.EntriesToSave = 20, nil
	require.ErrorIs(t, db.saveSnapshots([]pb.Update{ud}), errIterate)
}
