NAME=httproxy
OUTDIR=output
GOBUILD=CGO_ENABLED=0 go build -ldflags '-w -s'

linux:
	GOARCH=amd64 GOOS=linux $(GOBUILD) -o $(OUTDIR)/$(NAME)-$@

clean:
	rm -rf $(OUTDIR)
