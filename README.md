# Go Copilot Observability
This repo contains a simple instrumented [go](https://go.dev/) service along with infra to deploy it to AWS ECS and AWS App Runner using [Copilot](https://github.com/aws/copilot-cli).

# Requirements
- [AWS Copilot](https://github.com/aws/copilot-cli#installation)

# Deploy
1. Clone this repo.
1. Deploy this service using Copilot!
```bash
copilot init --app copilot-playground --name movies-lbws --type "Load Balanced Web Service" --deploy
```

# Generate and View Traces
Now that you have the movies service deployed, let's generate some traces! After deploying your service, copilot gives you a URL to access your service. Copy that URL, and replace `$MOVIES_SERVICE_BASE_URL` with it in the commands below.

First, let's insert [a movie](https://www.imdb.com/title/tt6751668/) into our database:
```bash
curl -X POST '$MOVIES_SERVICE_BASE_URL/movies/api/movie' -d '{"title": "Parasite", "year": 2019}'
```

In the response JSON, there should be an `id` field. Replace the below `MOVIE_ID` with that, and let's query the movie we just inserted:
```bash
curl -X GET '$MOVIES_SERVICE_BASE_URL/movies/api/movie?id=$MOVIE_ID'
```

Sweet! Now we should have some traces to view in the AWS Console. After logging in, go to CloudWatch and click on the _Service Map_ in the sidebar, under _X-Ray traces_. It should look something like this:
![Service Map](https://user-images.githubusercontent.com/10566468/167501064-bf641724-69f1-4128-ada5-6e631bc60cca.png)

Now switch to the _Traces_ tab in the sidebar, and you should be able to see your requests. If you open one up, you can see more details about that specific request like this:
![Trace Details](https://user-images.githubusercontent.com/10566468/167501126-6db85c36-57f5-4377-a1e1-8daccaed6184.png)