
build:
	docker build -t service-wrapper .

extract:
	rm -f service-wrapper
	docker run --name extract --rm service-wrapper sleep 1000 &
	sleep 1
	-docker cp extract:service-wrapper .
	docker kill extract
	[ -e service-wrapper ]
