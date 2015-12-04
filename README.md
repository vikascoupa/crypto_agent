# crypto_agent
This is a stub for Coupa Crypto Agent

The idea is that this agent will provide encryption, decryption, key generation, rotation, deletion and other key management features either by fronting a crypto endpoint or by implementing some of this functionality directly.

# Running
curl http://localhost:8080/v1/encrypt?cleartext=clearcase

curl http://localhost:8080/v1/decrypt?ciphertext="biSM40LNI55wY1ftFkHPQc2DhbrRHB3wh9wjUQ=="

# Load testing
vegeta attack -targets=./load.txt -rate 100 > ./results

cat results | vegeta report -reporter=json > ./metrics.json

cat results | vegeta report -reporter=plot > plot.html
