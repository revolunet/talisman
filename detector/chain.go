package detector

import (
	"os"
	"talisman/checksumcalculator"
	"talisman/detector/detector"
	"talisman/detector/filecontent"
	"talisman/detector/filename"
	"talisman/detector/helpers"
	"talisman/detector/pattern"
	"talisman/gitrepo"
	"talisman/talismanrc"
	"talisman/utility"

	"github.com/cheggaaa/pb/v3"
	log "github.com/sirupsen/logrus"
)

//Chain represents a chain of Detectors.
//It is itself a detector.
type Chain struct {
	detectors []detector.Detector
	hooktype  string
}

//NewChain returns an empty DetectorChain
//It is itself a detector, but it tests nothing.
func NewChain(hooktype string) *Chain {
	result := Chain{make([]detector.Detector, 0), hooktype}
	return &result
}

//DefaultChain returns a DetectorChain with pre-configured detectors
func DefaultChain(tRC *talismanrc.TalismanRC, hooktype string) *Chain {
	result := NewChain(hooktype)
	result.AddDetector(filename.DefaultFileNameDetector(tRC.Threshold))
	result.AddDetector(filecontent.NewFileContentDetector(tRC))
	result.AddDetector(pattern.NewPatternDetector(tRC.CustomPatterns))
	return result
}

//AddDetector adds the detector that is passed in to the chain
func (dc *Chain) AddDetector(d detector.Detector) *Chain {
	dc.detectors = append(dc.detectors, d)
	return dc
}

//Test validates the additions against each detector in the chain.
//The results are passed in from detector to detector and thus collect all errors from all detectors
func (dc *Chain) Test(currentAdditions []gitrepo.Addition, talismanRC *talismanrc.TalismanRC, result *helpers.DetectionResults) {
	wd, _ := os.Getwd()
	repo := gitrepo.RepoLocatedAt(wd)
	allAdditions := repo.TrackedFilesAsAdditions()
	var hasher utility.SHA256Hasher
	if dc.hooktype == "pre-push" {
		hasher = utility.NewGitHeadFileSHA256Hasher(wd)
	} else {
		hasher = utility.NewGitFileSHA256Hasher(wd)
	}
	calculator := checksumcalculator.NewChecksumCalculator(hasher, append(allAdditions, currentAdditions...))
	cc := helpers.NewChecksumCompare(calculator, hasher, talismanRC)
	log.Printf("Number of files to scan: %d\n", len(currentAdditions))
	log.Printf("Number of detectors: %d\n", len(dc.detectors))
	total := len(currentAdditions) * len(dc.detectors)
	var progressBar = getProgressBar()
	progressBar.Start(total)
	for _, v := range dc.detectors {
		v.Test(cc, currentAdditions, talismanRC, result, func() {
			progressBar.Increment()
		})
	}
	progressBar.Finish()
}

func getProgressBar() progressBar {
	if isTerminal() {
		return &defaultProgressBar{}
	} else {
		return &noOpProgressBar{}
	}
}

func isTerminal() bool {
	fileInfo, _ := os.Stdout.Stat()
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

type progressBar interface {
	Start(int)
	Increment()
	Finish()
}

type noOpProgressBar struct {
}

func (d *noOpProgressBar) Start(int) {}

func (d *noOpProgressBar) Increment() {}

func (d *noOpProgressBar) Finish() {}

type defaultProgressBar struct {
	bar *pb.ProgressBar
}

func (d *defaultProgressBar) Start(total int) {
	bar := pb.ProgressBarTemplate(`{{ red "Talisman Scan:" }} {{counters .}} {{ bar . "<" "-" (cycle . "↖" "↗" "↘" "↙" ) "." ">"}} {{percent . | rndcolor }} {{green}} {{blue}}`).New(total)
	bar.Set(pb.Terminal, true)
	d.bar = bar.Start()
}

func (d *defaultProgressBar) Increment() {
	d.bar.Increment()
}

func (d *defaultProgressBar) Finish() {
	d.bar.Finish()
}
