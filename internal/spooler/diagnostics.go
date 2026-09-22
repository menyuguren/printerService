package spooler

import "fmt"

func stageError(stage string, jobID uint32, err error) error {
	if jobID == 0 {
		return fmt.Errorf("%s: %w", stage, err)
	}
	return fmt.Errorf("%s job %d: %w", stage, jobID, err)
}
