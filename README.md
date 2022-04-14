# Go Copilot Observability
This repo contains a simple instrumented [go](https://go.dev/) service along with infra to deploy it to AWS ECS and AWS App Runner using [Copilot](https://github.com/aws/copilot-cli).

# Requirements
- [AWS Copilot](https://github.com/aws/copilot-cli#installation)

# Deploy
1. Clone this repo.
1. If you don't already have a copilot application, create one:
```bash
copilot init --app copilot-playground --name movies-lbws --deploy # TODO: do i need the type?
```

# Generate and View Traces
Now that you have the movies service deployed, let's generate some traces! After deploying your service, copilot gives you a URL to access your service at. Copy that URL, and replace `$MOVIES_SERVICE_BASE_URL` with it in the commands below.
These endpoints are instrumented using the Open Telemetry SDK, but you can change the `otel` part of the endpoint to `xray` to generate traces using the X-Ray SDK instead.

_Note: X-Ray SDK traces are not supported in Request-Driven Web Services_

First, let's insert [a movie](https://www.imdb.com/title/tt6751668/) into our database:
```bash
curl -X POST '$MOVIES_SERVICE_BASE_URL/movies/api/otel/movie' -d '{"title": "Parasite", "year": 2019}'
```

In the response JSON, there should be an `id` field. Replace the below `MOVIE_ID` with that, and let's query the movie we just inserted:
```bash
curl -X GET '$MOVIES_SERVICE_BASE_URL/movies/api/otel/movie?id=$MOVIE_ID'
```

Sweet! Now we should have some traces to view in the AWS Console. After logging in, go to CloudWatch and click on the _Service Map_ in the sidebar, under _X-Ray traces_. It should look something like this:
<!-- TODO: insert image -->

Now switch to the _Traces_ tab in the sidebar, and you should be able to see your requests. If you open one up, you can see more details about that specific request like this:

# What's Next?

# Additional Resources