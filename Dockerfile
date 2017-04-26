FROM golang:1.8
RUN go get -v github.com/stephen-soltesz/alertmanager-github-receiver/cmd/github_receiver
# RUN go get -v cmd/github_receiver
ENTRYPOINT ["/go/bin/github_receiver"]
