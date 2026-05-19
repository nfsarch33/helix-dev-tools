package installcheck

import "os"

type Status string

const (
	StatusPass    Status = "pass"
	StatusFail    Status = "fail"
	StatusMissing Status = "missing"
)

type CheckResult struct {
	Path   string
	Status Status
	Error  string
}

func CheckBinaries(paths []string) []CheckResult {
	results := make([]CheckResult, 0, len(paths))
	for _, p := range paths {
		info, err := os.Stat(p)
		if os.IsNotExist(err) {
			results = append(results, CheckResult{Path: p, Status: StatusMissing, Error: "file not found"})
			continue
		}
		if err != nil {
			results = append(results, CheckResult{Path: p, Status: StatusFail, Error: err.Error()})
			continue
		}
		if info.IsDir() {
			results = append(results, CheckResult{Path: p, Status: StatusFail, Error: "path is a directory, not a binary"})
			continue
		}
		results = append(results, CheckResult{Path: p, Status: StatusPass})
	}
	return results
}

func CheckConfigs(paths []string) []CheckResult {
	results := make([]CheckResult, 0, len(paths))
	for _, p := range paths {
		info, err := os.Stat(p)
		if os.IsNotExist(err) {
			results = append(results, CheckResult{Path: p, Status: StatusMissing, Error: "config not found"})
			continue
		}
		if err != nil {
			results = append(results, CheckResult{Path: p, Status: StatusFail, Error: err.Error()})
			continue
		}
		if info.IsDir() {
			results = append(results, CheckResult{Path: p, Status: StatusFail, Error: "path is a directory, not a config file"})
			continue
		}
		results = append(results, CheckResult{Path: p, Status: StatusPass})
	}
	return results
}
