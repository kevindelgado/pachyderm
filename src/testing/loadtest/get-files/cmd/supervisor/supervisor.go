package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/pachyderm/pachyderm/src/client"
	"github.com/pachyderm/pachyderm/src/client/pfs"
	"github.com/pachyderm/pachyderm/src/client/pps"
)

var (
	numFiles int64

	// commitTimes[i] is the amount of time that it took to start and finish
	// commit number 'i' (read by main() and PrintDurations())
	commitTime time.Duration

	// jobTimes[i] is the amount of time that it took to start and finish job
	// number 'i' (read by main() and PrintDurations())
	jobTime time.Duration
)

func init() {
	flag.Int64Var(&numFiles, "num-files", 100, "number of files "+
		"generated and put to input repo")
}

func PrintFlags() {
	log.Printf("num-files: %v\n", numFiles)
}

// func PrintDurations() {
// 	log.Print(" Commit Time    Job Time\n")
// 	//for i := 0; i < numCommits; i++ {
// 	// fmt.Printf(" %3d: ", i)
// 	// if i < len(commitTimes) {
// 	log.Printf("%11.3f", commitTime.Seconds())
// 	// } else {
// 	// fmt.Print("        ---")
// 	// }
// 	log.Print(" ")
// 	// if i < len(jobTimes) {
// 	log.Printf("%11.3f", jobTime.Seconds())
// 	// } else {
// 	// fmt.Print("        ---")
// 	// }
// 	log.Print("\n")
// 	//}
// }

func main() {
	flag.Parse()
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)

	// TODO(kdelga): validate flags
	PrintFlags()

	// Connect to pachyderm cluster
	log.Printf("starting to initialize pachyderm client")
	log.Printf("pachd address: \"%s:%s\"", os.Getenv("PACHD_SERVICE_HOST"),
		os.Getenv("PACHD_SERVICE_PORT"))
	c, err := client.NewInCluster()
	if err != nil {
		log.Fatalf("could not initialize Pachyderm client: %v", err)
	}

	// Make sure cluster is empty
	if ris, err := c.ListRepo(); err != nil || len(ris) > 0 {
		log.Fatalf("cluster must be empty before running the \"split\" loadtest")
	}

	// Create input repo and pipeline
	log.Printf("creating input repo and pipeline")
	repo, branch := "input", "master"
	if err := c.CreateRepo(repo); err != nil {
		log.Fatalf("could not create input repo: %v", err)
	}
	_, err = c.PpsAPIClient.CreatePipeline(
		context.Background(),
		&pps.CreatePipelineRequest{
			Pipeline: &pps.Pipeline{Name: "get-files"},
			Transform: &pps.Transform{
				Image: "",
				Cmd:   []string{"bash"},
				Stdin: []string{
					fmt.Sprintf("cp -r /pfs/%s/ /pfs/out/", repo),
				},
			},
			ParallelismSpec: &pps.ParallelismSpec{Constant: 1},
			ResourceRequests: &pps.ResourceSpec{
				Memory: "1G",
				Cpu:    1,
			},
			Input: &pps.Input{
				Pfs: &pps.PFSInput{
					Repo:   repo,
					Branch: branch,
					Glob:   "/",
				},
			},
		},
	)
	if err != nil {
		log.Fatalf("could not create load test pipeline: %v", err)
	}

	commitStart := time.Now()
	// Start commit
	commit, err := c.StartCommit(repo, branch)
	if err != nil {
		log.Fatalf("could not start commit: %v", err)
	}
	log.Printf("starting commit (%s)", commit.ID)
	pfclient, err := c.PfsAPIClient.PutFile(context.Background())
	for i := int64(0); i < numFiles; i++ {
		name := fmt.Sprintf("input-%d", i)
		if err := pfclient.Send(&pfs.PutFileRequest{
			File:  client.NewFile(repo, branch, name),
			Value: []byte(name),
		}); err != nil {
			log.Fatalf("error from put-file: %v", err, "\nfile: ", name)
		}
	}
	_, err = pfclient.CloseAndRecv()
	if err != nil {
		log.Fatalf("err closing pfclient: %v", err)
	}

	if err := c.FinishCommit(repo, commit.ID); err != nil {
		log.Fatalf("could not finish commit: %v", err)
	}
	jobStart := time.Now()
	commitTime = jobStart.Sub(commitStart)
	log.Printf("commit (%s) finished", commit.ID)

	iter, err := c.FlushCommit([]*pfs.Commit{commit}, []*pfs.Repo{client.NewRepo("get-files-load")})
	if err != nil {
		log.Fatalf("could not flush commit %v", err)
	}
	if _, err = iter.Next(); err != nil && err != io.EOF {
		log.Fatalf("could not get commit info after flushing commit %v", err)
	}
	jobTime = time.Now().Sub(jobStart)
	log.Printf("job (commit %s) finished", commit.ID)

	//TODO(kdelga): validate output
	// PrintDurations()
	log.Println("[ Pass ] commitTime: ", commitTime, "jobTime: ", jobTime)

}
