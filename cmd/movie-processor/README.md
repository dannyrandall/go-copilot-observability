# Movie Processor

The movie processor is an example [worker service]().

It polls a queue (Amazon SQS) for new movies to create, and then calls a [backend service] to create
the movie.

All of these calls are instrumented using Open Telemetry, and can be viewed in the AWS CloudWatch X-Ray console.