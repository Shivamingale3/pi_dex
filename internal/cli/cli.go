package cli

import "os"

type Command struct {
	Action string
	Event  string
	DryRun bool
}

func Parse() Command {

	cmd := Command{}

	args := os.Args

	if len(args) < 3 {
		return cmd
	}

	cmd.Action = args[1]
	cmd.Event = args[2]

	for _, arg := range args {

		if arg == "--dry-run" {
			cmd.DryRun = true
		}
	}

	return cmd
}