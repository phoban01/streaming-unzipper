.PHONY: compile zip build deploy

FUNC=s3unzip
SRC=main.go

$(FUNC): $(SRC)
	@GOOS=linux CGO_ENABLED=0 go build -o $(FUNC) $(SRC)

zip: $(FUNC)
	zip -q deploy.zip $(FUNC)

build: zip
	sam build

deploy: build
	sam deploy --profile ${AWS_PROFILE}
