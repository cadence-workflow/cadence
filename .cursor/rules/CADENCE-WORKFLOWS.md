When Implementing Cadence Workflows, follow these guidelines:

## Avoid Iterating over Go's native map to prevent non-determinism during replays.

Native Go map iteration is non-deterministic since the iteration order of a map is random and can vary across executions, thus breaking the determinism in Cadence workflows needed for workflow replays and fault tolerance. Consequently, the map must be converted into a deterministic structure before iterating.

- Good Code Example:

keys := make([]string, 0, len(myMap))for key := range myMap {
    keys = append(keys, key)
}
sort.Strings(keys)
for _, key := range keys {
    value := myMap[key]
    workflow.GetLogger(ctx).Info("Key:", key, "Value:", value)
}

- Bad Code Example:

for key, value := range myMap {
    workflow.GetLogger(ctx).Info("Key:", key, "Value:", value)
}

## Avoid using Go's goroutine. Use workflow.Go

In Cadence, using goroutine inside workflows is generally avoided because goroutines are not tracked by the Cadence runtime. Using workflow.Go creates a Cadence-managed concurrent thread that operates within the workflow’s deterministic execution model, ensuring event replayability and consistency.

- Good Code Example:

workflow.Go(ctx, func(ctx workflow.Context) {
    activityInput := "process this"
    err := workflow.ExecuteActivity(ctx, YourActivity, activityInput).Get(ctx, nil)
    if err != nil {
        workflow.GetLogger(ctx).Error("Activity failed.", "Error", err)
    }
})

- Bad Code Example:

go func() {
    err := YourExternalFunction()
    if err != nil {
        log.Println("Something went wrong:", err)
    }
}()

## Limit Concurrency of Activities and Child workflows

Running many activities or child workflows concurrently can disrupt stability and resource efficiency by overwhelming the workers. Approaches that limit concurrency should be employed to ensure that only a fixed number of tasks run in parallel, preserving determinism and avoiding overload.

- Good Code Example:

const maxConcurrent = 3
semaphore := make(chan struct{}, maxConcurrent)
for _, input := range inputs {
    semaphore <- struct{}{}
    workflow.Go(ctx, func(ctx workflow.Context) {
        defer func() { <-semaphore }()
        err := workflow.ExecuteActivity(ctx, YourActivity, input).Get(ctx, nil)
        if err != nil {
            workflow.GetLogger(ctx).Error("Activity failed", "Error", err)
        }
    })
}

- Bad Code Example:

for _, input := range inputs {
    workflow.Go(ctx, func(ctx workflow.Context) {
        err := workflow.ExecuteActivity(ctx, YourActivity, input).Get(ctx, nil)
        if err != nil {
            workflow.GetLogger(ctx).Error("Activity failed", "Error", err)
        }
    })
}

## Avoid Using time.Now()

Using time.Now() inside Cadence workflows introduces non-determinism since Cadence relies on event sourcing and replay to ensure fault tolerance. This method returns the actual system time, which varies between executions. Instead, it is advised to use workflow.Now(ctx) as it returns a deterministic timestamp based on the workflow’s event history.

- Good Code Example:

now := workflow.Now(ctx)
workflow.GetLogger(ctx).Info("Current time:", zap.Time("timestamp", now))

- Bad Code Example:

now := time.Now()
workflow.GetLogger(ctx).Info("Current time:", zap.Time("timestamp", now))

## Avoid Using Dynamic Signal Names

Using dynamic signal names can lead to non-deterministic behavior since Cadence tracks signals by name in the workflow history, making it difficult to replay workflows reliably, query signal state, or handle signals consistently. Instead, define static signal names in the workflow interface.

- Good Code Example:

func MyWorkflow(ctx workflow.Context) error {
    signalChan := workflow.GetSignalChannel(ctx, "statusUpdate")
    workflow.Go(ctx, func(ctx workflow.Context) {
        var status string
        for {
            signalChan.Receive(ctx, &status)
            workflow.GetLogger(ctx).Info("Received status:", "status", status)
        }
    })
    return nil
}

- Bad Code Example:

func MyWorkflow(ctx workflow.Context, userID string) error {
    signalName := "signal_" + userID
    signalChan := workflow.GetSignalChannel(ctx, signalName)
    return nil
}

## Limit use of Same Workflow-ID for frequent workflow returns

Reusing the same workflow-id for frequency or continuous runs can lead to a hot shard problem in Cadence. Since workflow-ids are used for partitioning, repeated use of the same workflow-id can create a performance bottleneck on that specific shard. Instead, it is advised to use unique or distributed workflow-ids to ensure that the load is spread evenly across the shards.

- Good Code Example:

workflowOptions := client.StartWorkflowOptions{
    ID:        fmt.Sprintf("order-workflow-%d", time.Now().UnixNano()),
    TaskQueue: "orderQueue",
}

we, err := c.ExecuteWorkflow(context.Background(), workflowOptions, OrderWorkflow, orderData)

- Bad Code Example:

workflowOptions := client.StartWorkflowOptions{
    ID:        "order-workflow",
    TaskQueue: "orderQueue",
}

we, err := c.ExecuteWorkflow(context.Background(), workflowOptions, OrderWorkflow, orderData)

## Avoid using time.Sleep()

Using time.Sleep() in a Cadence workflow breaks determinism since it relies on real system time. Thus, time.Sleep() can behave differently during replays, leading to inconsistent outcomes. Instead, use workflow.Sleep(ctx, duration), which is managed by the Cadence runtime and ensures consistent behavior across executions and replays.

- Good Code Example:

func MyWorkflow(ctx workflow.Context) error {
    workflow.GetLogger(ctx).Info("Sleeping for 10 seconds...")
    err := workflow.Sleep(ctx, 10*time.Second)
    if err != nil {
        return err
    }
    workflow.GetLogger(ctx).Info("Woke up after sleep")
    return nil
}

- Bad Code Example:

func MyWorkflow(ctx workflow.Context) error {
    log.Println("Sleeping for 10 seconds...")
    time.Sleep(10 * time.Second)
    log.Println("Woke up after sleep")
    return nil
}

## Register Workflows and Activities with a String name and use those when starting

Registering workflows and activities using explicit string names allows them to be started by their name instead of through function reference. This improves decoupling and ensures that workers and clients can communicate using consistent identifiers.

- Good Code Example:

activity.RegisterWithOptions(MyActivityFunc, activity.RegisterOptions{Name: "MyActivity"})
workflow.RegisterWithOptions(MyWorkflowFunc, workflow.RegisterOptions{Name: "MyWorkflow"})
workflowOptions := client.StartWorkflowOptions{
    ID:        "my-workflow-id",
    TaskList:  "my-task-list",
}
client.ExecuteWorkflow(ctx, workflowOptions, "MyWorkflow", inputData)

- Bad Code Example:

workflow.Register(MyWorkflowFunc)
activity.Register(MyActivityFunc)
client.ExecuteWorkflow(ctx, workflowOptions, MyWorkflowFunc, inputData)