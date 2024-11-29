package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"sync"
)

type Job struct {
	ID int `json:"id"`
}

func worker(server string, project_id int, token string, jobs <-chan Job, results chan<- *http.Response, wg *sync.WaitGroup) {
	for job := range jobs {
		fmt.Println("job to process:", job.ID)
		url := fmt.Sprintf("%v/api/v4/projects/%v/jobs/%d/artifacts", server, project_id, job.ID)
		req, err := http.NewRequest("DELETE", url, nil)
		if err != nil {
			fmt.Println(err)
			return
		}
		req.Header.Set("PRIVATE-TOKEN", token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Println(err)
			return
		}
		defer resp.Body.Close()
		fmt.Println("job artifacts deleted:", job.ID, resp.StatusCode)

		results <- resp
		wg.Done()
	}
}

func main() {
	var project_id int
	var per_page int
	var pages int
	var concurrent_requests int
	var token string
	var server string

	flag.IntVar(&project_id, "project_id", 0, "Project ID")
	flag.IntVar(&per_page, "per_page", 100, "Number of jobs per page")
	flag.IntVar(&pages, "pages", 1, "Number of pages")
	flag.IntVar(&concurrent_requests, "concurrent_requests", 5, "Number of concurrent requests")
	flag.StringVar(&token, "token", "", "Private token")
	flag.StringVar(&server, "server", "", "Gitlab server")
	flag.Parse()

	fmt.Printf("Fetching %d pages from %v\n", pages, server)

	for i := 1; i <= pages; i++ {
		url := fmt.Sprintf("%v/api/v4/projects/%v/jobs?per_page=%d&page=%d&artifact_expired=false", server, project_id, per_page, i)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			fmt.Println(err)
			return
		}
		println("page number:", i)
		req.Header.Set("PRIVATE-TOKEN", token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Println(err)
			return
		}
		defer resp.Body.Close()

		var jobs []Job
		err = json.NewDecoder(resp.Body).Decode(&jobs)
		if err != nil {
			fmt.Println(err)
			return
		}

		pool_jobs := make(chan Job, len(jobs))
		results := make(chan *http.Response, len(jobs))
		var wg sync.WaitGroup

		// Start workers
		for w := 0; w < concurrent_requests; w++ {
			go worker(server, project_id, token, pool_jobs, results, &wg)
		}

		// Sending jobs to the worker pool
		wg.Add(len(jobs))
		for _, job := range jobs {
			pool_jobs <- job
		}
		close(pool_jobs)
		wg.Wait()

	}
}
