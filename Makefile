
build:
	docker build -t exec .

extract:
	rm -f service-wrapper
	docker run --name extract --rm exec sleep 1000 &
	sleep 1
	-docker cp extract:service-wrapper .
	docker kill extract
	[ -e service-wrapper ]
