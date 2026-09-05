package cli

// import "fmt"

// func helpHandler(*Args) error {
// 	fmt.Println("harness: a help guide")
// 	return nil
// }
//
// func (c *Cli) queryHandler(args *Args) error {
// 	if err := c.agent.Step(args.Positional[0]); err != nil {
// 		return fmt.Errorf("step error: %w", err)
// 	}
// 	fmt.Println(c.agent.Result())
// 	return nil
// }
//
// func (c *Cli) listTasksHandler(*Args) error {
// 	jobs, err := c.store.GetJobs()
// 	if err != nil {
// 		return err
// 	}
//
// 	for _, j := range jobs {
// 		status, result := j.Snapshot()
// 		fmt.Printf("#%d [%s] %s -> %q\n", j.Id, status, j.Instruction, result)
// 	}
//
// 	return nil
// }
