#!/usr/bin/env bash
# docker build --rm -f Dockerfile-binary -t 'slotix/dfk-binary' .
# docker build --rm -f cmd/fetch.d/Dockerfile -t 'slotix/dfk-fetch' .
# docker build --rm -f cmd/parse.d/Dockerfile -t 'slotix/dfk-parse' .
cd cmd/parse.d && make push
cd ../fetch.d && make push
cd ../../testserver && make push



#run tests here... 
#./test.sh

#docker push slotix/dfk-fetch
#docker push slotix/dfk-parse
#docker push slotix/dfk-testserver