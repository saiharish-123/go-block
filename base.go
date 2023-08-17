package main

// #include "rocksdb/c.h"
import (
	"flag"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"example.com/bchain"
	"example.com/bchain/coins"
	"example.com/common"
	"example.com/db"
	"github.com/golang/glog"
)

// debounce too close requests for resync
const debounceResyncIndexMs = 1009

// debounce too close requests for resync mempool (ZeroMQ sends message for each tx, when new block there are many transactions)
const debounceResyncMempoolMs = 1009

// store internal state about once every minute
const storeInternalStatePeriodMs = 59699

// exit codes from the main function
const exitCodeOK = 0
const exitCodeFatal = 255

var (
	configFile = flag.String("blockchaincfg", "", "path to blockchain RPC service configuration json file")

	dbPath         = flag.String("datadir", "./mydb", "path to database directory")
	dbCache        = flag.Int("dbcache", 1<<29, "size of the rocksdb cache")
	dbMaxOpenFiles = flag.Int("dbmaxopenfiles", 1<<14, "max open files by rocksdb")

	blockFrom      = flag.Int("blockheight", -1, "height of the starting block")
	blockUntil     = flag.Int("blockuntil", -1, "height of the final block")
	rollbackHeight = flag.Int("rollback", -1, "rollback to the given height and quit")

	synchronize = flag.Bool("sync", false, "synchronizes until tip, if together with zeromq, keeps index synchronized")
	repair      = flag.Bool("repair", false, "repair the database")
	fixUtxo     = flag.Bool("fixutxo", false, "check and fix utxo db and exit")
	prof        = flag.String("prof", "", "http server binding [address]:port of the interface to profiling data /debug/pprof/ (default no profiling)")

	syncChunk   = flag.Int("chunk", 100, "block chunk size for processing in bulk mode")
	syncWorkers = flag.Int("workers", 8, "number of workers to process blocks in bulk mode")
	dryRun      = flag.Bool("dryrun", false, "do not index blocks, only download")

	debugMode = flag.Bool("debug", false, "debug mode, return more verbose errors, reload templates on each request")

	internalBinding = flag.String("internal", "", "internal http server binding [address]:port, (default no internal server)")

	publicBinding = flag.String("public", "", "public http server binding [address]:port[/path] (default no public server)")

	certFiles = flag.String("certfile", "", "to enable SSL specify path to certificate files without extension, expecting <certfile>.crt and <certfile>.key (default no SSL)")

	explorerURL = flag.String("explorer", "", "address of blockchain explorer")

	noTxCache = flag.Bool("notxcache", false, "disable tx cache")

	enableSubNewTx = flag.Bool("enablesubnewtx", false, "enable support for subscribing to all new transactions")

	computeColumnStats  = flag.Bool("computedbstats", false, "compute column stats and exit")
	computeFeeStatsFlag = flag.Bool("computefeestats", false, "compute fee stats for blocks in blockheight-blockuntil range and exit")
	dbStatsPeriodHours  = flag.Int("dbstatsperiod", 24, "period of db stats collection in hours, 0 disables stats collection")

	// resync index at least each resyncIndexPeriodMs (could be more often if invoked by message from ZeroMQ)
	resyncIndexPeriodMs = flag.Int("resyncindexperiod", 935093, "resync index period in milliseconds")

	// resync mempool at least each resyncMempoolPeriodMs (could be more often if invoked by message from ZeroMQ)
	resyncMempoolPeriodMs = flag.Int("resyncmempoolperiod", 60017, "resync mempool period in milliseconds")

	extendedIndex = flag.Bool("extendedindex", false, "if true, create index of input txids and spending transactions")
)

var (
	chanSyncIndex              = make(chan struct{})
	chanSyncMempool            = make(chan struct{})
	chanStoreInternalState     = make(chan struct{})
	chanSyncIndexDone          = make(chan struct{})
	chanSyncMempoolDone        = make(chan struct{})
	chanStoreInternalStateDone = make(chan struct{})
	// chain                         bchain.BlockChain
	// mempool                       bchain.Mempool
	// index   *db.RocksDB
	// txCache *db.TxCache
	// metrics                       *common.Metrics
	// syncWorker *db.SyncWorker
	// internalState                 *common.InternalState
	// fiatRates                     *fiat.FiatRates
	// callbacksOnNewBlock           []bchain.OnNewBlockFunc
	// callbacksOnNewTxAddr          []bchain.OnNewTxAddrFunc
	// callbacksOnNewTx              []bchain.OnNewTxFunc
	// callbacksOnNewFiatRatesTicker []fiat.OnNewFiatRatesTicker
	chanOsSignal chan os.Signal
)

func init() {
	glog.MaxSize = 1024 * 1024 * 8
	glog.CopyStandardLogTo("INFO")
}

func main() {
	defer func() {
		if e := recover(); e != nil {
			glog.Error("main recovered from panic: ", e)
			debug.PrintStack()
			os.Exit(-1)
		}
	}()
	os.Exit(mainWithExitCode())
}

func mainWithExitCode() int {
	flag.Parse()

	defer glog.Flush()

	rand.Seed(time.Now().UTC().UnixNano())

	chanOsSignal = make(chan os.Signal, 1)
	signal.Notify(chanOsSignal, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)

	glog.Infof("Blockbook: %+v, debug mode %v", common.GetVersionInfo(), *debugMode)

	if *prof != "" {
		go func() {
			log.Println(http.ListenAndServe(*prof, nil))
		}()
	}

	if *repair {
		if err := db.RepairRocksDB(*dbPath); err != nil {
			glog.Errorf("RepairRocksDB %s: %v", *dbPath, err)
			return exitCodeFatal
		}
		return exitCodeOK
	}

	if *configFile == "" {
		glog.Error("Missing blockchaincfg configuration parameter")
		return exitCodeFatal
	}

	configFileContent, err := os.ReadFile(*configFile)
	if err != nil {
		glog.Errorf("Error reading file %v, %v", configFile, err)
		return exitCodeFatal
	}

	coin, coinShortcut, coinLabel, err := coins.GetCoinNameFromConfig(configFileContent)
	if err != nil {
		glog.Error("config: ", err)
		return exitCodeFatal
	}

	metrics, err = common.GetMetrics(coin)
	if err != nil {
		glog.Error("metrics: ", err)
		return exitCodeFatal
	}

	if chain, mempool, err = getBlockChainWithRetry(coin, *configFile, pushSynchronizationHandler, metrics, 120); err != nil {
		glog.Error("rpc: ", err)
		return exitCodeFatal
	}

	index, err = db.NewRocksDB(*dbPath, *dbCache, *dbMaxOpenFiles, chain.GetChainParser(), metrics, *extendedIndex)
	if err != nil {
		glog.Error("rocksDB: ", err)
		return exitCodeFatal
	}
	defer index.Close()

	internalState, err = newInternalState(coin, coinShortcut, coinLabel, index, *enableSubNewTx)
	if err != nil {
		glog.Error("internalState: ", err)
		return exitCodeFatal
	}

	// fix possible inconsistencies in the UTXO index
	if *fixUtxo || !internalState.UtxoChecked {
		err = index.FixUtxos(chanOsSignal)
		if err != nil {
			glog.Error("fixUtxos: ", err)
			return exitCodeFatal
		}
		internalState.UtxoChecked = true
	}

	// sort addressContracts if necessary
	if !internalState.SortedAddressContracts {
		err = index.SortAddressContracts(chanOsSignal)
		if err != nil {
			glog.Error("sortAddressContracts: ", err)
			return exitCodeFatal
		}
		internalState.SortedAddressContracts = true
	}

	index.SetInternalState(internalState)
	if *fixUtxo {
		err = index.StoreInternalState(internalState)
		if err != nil {
			glog.Error("StoreInternalState: ", err)
			return exitCodeFatal
		}
		return exitCodeOK
	}

	if internalState.DbState != common.DbStateClosed {
		if internalState.DbState == common.DbStateInconsistent {
			glog.Error("internalState: database is in inconsistent state and cannot be used")
			return exitCodeFatal
		}
		glog.Warning("internalState: database was left in open state, possibly previous ungraceful shutdown")
	}

	if *computeFeeStatsFlag {
		internalState.DbState = common.DbStateOpen
		err = computeFeeStats(chanOsSignal, *blockFrom, *blockUntil, index, chain, txCache, internalState, metrics)
		if err != nil && err != db.ErrOperationInterrupted {
			glog.Error("computeFeeStats: ", err)
			return exitCodeFatal
		}
		return exitCodeOK
	}

	if *computeColumnStats {
		internalState.DbState = common.DbStateOpen
		err = index.ComputeInternalStateColumnStats(chanOsSignal)
		if err != nil {
			glog.Error("internalState: ", err)
			return exitCodeFatal
		}
		glog.Info("DB size on disk: ", index.DatabaseSizeOnDisk(), ", DB size as computed: ", internalState.DBSizeTotal())
		return exitCodeOK
	}

	syncWorker, err = db.NewSyncWorker(index, chain, *syncWorkers, *syncChunk, *blockFrom, *dryRun, chanOsSignal, metrics, internalState)
	if err != nil {
		glog.Errorf("NewSyncWorker %v", err)
		return exitCodeFatal
	}

	// set the DbState to open at this moment, after all important workers are initialized
	internalState.DbState = common.DbStateOpen
	err = index.StoreInternalState(internalState)
	if err != nil {
		glog.Error("internalState: ", err)
		return exitCodeFatal
	}

	if *rollbackHeight >= 0 {
		err = performRollback()
		if err != nil {
			return exitCodeFatal
		}
		return exitCodeOK
	}

	if txCache, err = db.NewTxCache(index, chain, metrics, internalState, !*noTxCache); err != nil {
		glog.Error("txCache ", err)
		return exitCodeFatal
	}

	if fiatRates, err = fiat.NewFiatRates(index, configFileContent, metrics, onNewFiatRatesTicker); err != nil {
		glog.Error("fiatRates ", err)
		return exitCodeFatal
	}

	// report BlockbookAppInfo metric, only log possible error
	if err = blockbookAppInfoMetric(index, chain, txCache, internalState, metrics); err != nil {
		glog.Error("blockbookAppInfoMetric ", err)
	}

	var internalServer *server.InternalServer
	if *internalBinding != "" {
		internalServer, err = startInternalServer()
		if err != nil {
			glog.Error("internal server: ", err)
			return exitCodeFatal
		}
	}

	var publicServer *server.PublicServer
	if *publicBinding != "" {
		publicServer, err = startPublicServer()
		if err != nil {
			glog.Error("public server: ", err)
			return exitCodeFatal
		}
	}

	if *synchronize {
		internalState.SyncMode = true
		internalState.InitialSync = true
		if err := syncWorker.ResyncIndex(nil, true); err != nil {
			if err != db.ErrOperationInterrupted {
				glog.Error("resyncIndex ", err)
				return exitCodeFatal
			}
			return exitCodeOK
		}
		// initialize mempool after the initial sync is complete
		var addrDescForOutpoint bchain.AddrDescForOutpointFunc
		if chain.GetChainParser().GetChainType() == bchain.ChainBitcoinType {
			addrDescForOutpoint = index.AddrDescForOutpoint
		}
		err = chain.InitializeMempool(addrDescForOutpoint, onNewTxAddr, onNewTx)
		if err != nil {
			glog.Error("initializeMempool ", err)
			return exitCodeFatal
		}
		var mempoolCount int
		if mempoolCount, err = mempool.Resync(); err != nil {
			glog.Error("resyncMempool ", err)
			return exitCodeFatal
		}
		internalState.FinishedMempoolSync(mempoolCount)
		go syncIndexLoop()
		go syncMempoolLoop()
		internalState.InitialSync = false
	}
	go storeInternalStateLoop()

	if publicServer != nil {
		// start full public interface
		callbacksOnNewBlock = append(callbacksOnNewBlock, publicServer.OnNewBlock)
		callbacksOnNewTxAddr = append(callbacksOnNewTxAddr, publicServer.OnNewTxAddr)
		callbacksOnNewTx = append(callbacksOnNewTx, publicServer.OnNewTx)
		callbacksOnNewFiatRatesTicker = append(callbacksOnNewFiatRatesTicker, publicServer.OnNewFiatRatesTicker)
		publicServer.ConnectFullPublicInterface()
	}

	if *blockFrom >= 0 {
		if *blockUntil < 0 {
			*blockUntil = *blockFrom
		}
		height := uint32(*blockFrom)
		until := uint32(*blockUntil)

		if !*synchronize {
			if err = syncWorker.ConnectBlocksParallel(height, until); err != nil {
				if err != db.ErrOperationInterrupted {
					glog.Error("connectBlocksParallel ", err)
					return exitCodeFatal
				}
				return exitCodeOK
			}
		}
	}

	if internalServer != nil || publicServer != nil || chain != nil {
		// start fiat rates downloader only if not shutting down immediately
		initDownloaders(index, chain, configFileContent)
		waitForSignalAndShutdown(internalServer, publicServer, chain, 10*time.Second)
	}

	if *synchronize {
		close(chanSyncIndex)
		close(chanSyncMempool)
		close(chanStoreInternalState)
		<-chanSyncIndexDone
		<-chanSyncMempoolDone
		<-chanStoreInternalStateDone
	}
	return exitCodeOK
}
