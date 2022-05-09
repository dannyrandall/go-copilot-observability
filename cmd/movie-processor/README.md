# Movie Processor

The movie processor is an example [worker service](https://aws.github.io/copilot-cli/docs/concepts/services#request-driven-web-service).

It polls a queue (Amazon SQS) for new movies to create, and then calls the [backend service](../movies) to create the movie.

All of these calls are instrumented using Open Telemetry, and can be viewed in the AWS CloudWatch X-Ray console.