package main

import (
	"compress/gzip"
	"flag"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/dgraph-io/dgraph/loader"
	"github.com/dgraph-io/dgraph/posting"
	"github.com/dgraph-io/dgraph/store"
	"github.com/dgraph-io/dgraph/x"
)

var glog = x.Log("uidassigner_main")

var rdfGzips = flag.String("rdfgzips", "",
	"Comma separated gzip files containing RDF data")
var instanceIdx = flag.Uint64("instanceIdx", 0, "Only pick entities, where Fingerprint % numInstance == instanceIdx.")
var numInstance = flag.Uint64("numInstance", 1, "Total number of instances among which uid assigning is shared")
var uidDir = flag.String("uidpostings", "", "Directory to store xid to uid posting lists")
var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
var numcpu = flag.Int("numCpu", runtime.NumCPU(), "Number of cores to be used by the process")

func main() {
	flag.Parse()
	if !flag.Parsed() {
		glog.Fatal("Unable to parse flags")
	}
	if len(*cpuprofile) > 0 {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			glog.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	logrus.SetLevel(logrus.InfoLevel)
	numCpus := *numcpu
	prevProcs := runtime.GOMAXPROCS(numCpus)
	glog.WithField("num_cpus", numCpus).
		WithField("prev_maxprocs", prevProcs).
		Info("Set max procs to num  cpus")

	if len(*rdfGzips) == 0 {
		glog.Fatal("No RDF GZIP files specified")
	}

	ps := new(store.Store)
	ps.Init(*uidDir)
	defer ps.Close()

	posting.Init(ps, nil)

	files := strings.Split(*rdfGzips, ",")
	for _, path := range files {
		if len(path) == 0 {
			continue
		}
		glog.WithField("path", path).Info("Handling...")
		f, err := os.Open(path)
		if err != nil {
			glog.WithError(err).Fatal("Unable to open rdf file.")
		}

		r, err := gzip.NewReader(f)
		if err != nil {
			glog.WithError(err).Fatal("Unable to create gzip reader.")
		}

		count, err := loader.HandleRdfReaderWhileAssign(r, *instanceIdx, *numInstance)
		if err != nil {
			glog.WithError(err).Fatal("While handling rdf reader.")
		}
		glog.WithField("count", count).Info("RDFs parsed")
		r.Close()
		f.Close()
	}
	glog.Info("Calling merge lists")
	posting.MergeLists(100 * numCpus) // 100 per core.
}
