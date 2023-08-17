package db

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/tecbot/gorocksdb"
)

func RepairRocksDB(name string) error {
	glog.Infof("rocksdb: repair")
	opts := gorocksdb.NewDefaultOptions()
	return gorocksdb.RepairDb(name, opts)
}

func DatabaseStart() (*gorocksdb.DB, error) {
	opts := gorocksdb.NewDefaultOptions()
	defer opts.Destroy()
	opts.SetCreateIfMissing(true)
	db, err := gorocksdb.OpenDb(opts, "mydb")
	if err != nil {
		fmt.Println("Error opening database:", err)
		return nil, err
	}
	return db, nil
}

// package db

// import (
// 	"github.com/linxGnu/grocksdb"
// )

// type RocksDB struct {
// 	path string
// 	db   *grocksdb.DB
// 	wo   *grocksdb.WriteOptions
// 	ro   *grocksdb.ReadOptions
// 	cfh  []*grocksdb.ColumnFamilyHandle
// 	// chainParser   bchain.BlockChainParser
// 	// is            *common.InternalState
// 	// metrics       *common.Metrics
// 	cache        *grocksdb.Cache
// 	maxOpenFiles int
// 	// cbs           connectBlockStats
// 	extendedIndex bool
// }

// var cfNames []string

// // func createAndSetDBOptions(bloomBits int, c *grocksdb.Cache, maxOpenFiles int) *grocksdb.Options {
// // 	blockOpts := grocksdb.NewDefaultBlockBasedTableOptions()
// // 	blockOpts.SetBlockSize(32 << 10) // 32kB
// // 	blockOpts.SetBlockCache(c)
// // 	if bloomBits > 0 {
// // 		blockOpts.SetFilterPolicy(grocksdb.NewBloomFilter(float64(bloomBits)))
// // 	}
// // 	blockOpts.SetFormatVersion(4)

// // 	opts := grocksdb.NewDefaultOptions()
// // 	opts.SetBlockBasedTableFactory(blockOpts)
// // 	opts.SetCreateIfMissing(true)
// // 	opts.SetCreateIfMissingColumnFamilies(true)
// // 	opts.SetMaxBackgroundCompactions(6)
// // 	opts.SetMaxBackgroundFlushes(6)
// // 	opts.SetBytesPerSync(8 << 20)         // 8MB
// // 	opts.SetWriteBufferSize(1 << 27)      // 128MB
// // 	opts.SetMaxBytesForLevelBase(1 << 27) // 128MB
// // 	opts.SetMaxOpenFiles(maxOpenFiles)
// // 	// opts.SetCompression(grocksdb.LZ4HCCompression)
// // 	return opts
// // }

// func openDB(path string, c *grocksdb.Cache, openFiles int) (*grocksdb.DB, []*grocksdb.ColumnFamilyHandle, error) {
// 	// opts with bloom filter
// 	// opts := createAndSetDBOptions(10, c, openFiles)
// 	opts := grocksdb.NewDefaultOptions()
// 	// opts for addresses without bloom filter
// 	// from documentation: if most of your queries are executed using iterators, you shouldn't set bloom filter
// 	// optsAddresses := createAndSetDBOptions(0, c, openFiles)
// 	// default, height, addresses, blockTxids, transactions
// 	cfOptions := []*grocksdb.Options{opts, opts, opts, opts, opts, opts}
// 	// append type specific options
// 	count := len(cfNames) - len(cfOptions)
// 	for i := 0; i < count; i++ {
// 		cfOptions = append(cfOptions, opts)
// 	}
// 	// grocksdb.ListColumnFamilies()
// 	db, cfh, err := grocksdb.OpenDbColumnFamilies(opts, path, cfNames, cfOptions)
// 	if err != nil {
// 		return nil, nil, err
// 	}
// 	return db, cfh, nil
// }

// func NewRocksDB(path string, cacheSize, maxOpenFiles int) (d *RocksDB, err error) {
// 	cfNames = append([]string{"default", "tx", "ERC721", "ERC1155", "else"})
// 	c := grocksdb.NewLRUCache(uint64(cacheSize))
// 	db, cfh, err := openDB(path, c, maxOpenFiles)
// 	if err != nil {
// 		return nil, err
// 	}
// 	wo := grocksdb.NewDefaultWriteOptions()
// 	ro := grocksdb.NewDefaultReadOptions()
// 	return &RocksDB{path, db, wo, ro, cfh, c, maxOpenFiles, false}, nil
// }

// func DatabaseStart() (*grocksdb.DB, error) {
// 	bbto := grocksdb.NewDefaultBlockBasedTableOptions()
// 	bbto.SetBlockCache(grocksdb.NewLRUCache(3 << 30))

// 	opts := grocksdb.NewDefaultOptions()
// 	opts.SetBlockBasedTableFactory(bbto)
// 	opts.SetCreateIfMissing(true)

// 	db, err := grocksdb.OpenDb(opts, "/data")
// 	return db, err
// }
