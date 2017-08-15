HOST=$1
if [ ! -z $HOST ];then
    IP=$(docker-machine ip $HOST)
else
    IP=localhost
fi
curl -X POST -H 'content-type: application/json' -d @test.json http://$IP:3000/jobs
