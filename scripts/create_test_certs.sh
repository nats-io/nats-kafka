#!/bin/bash
# References:
# http://docs.confluent.io/2.0.0/kafka/ssl.html
# http://stackoverflow.com/questions/2846828/converting-jks-to-p12
# https://raw.githubusercontent.com/trastle/docker-kafka-ssl/master/generate-docker-kafka-ssl-certs.sh

PASSWORD="password"

SERVER_JKS="server.jks"
SERVER_P12="server.p12"
SERVER_PEM="server.pem"

CLIENT_JKS="client.jks"
CLIENT_P12="client.p12"
CLIENT_PEM="client.pem"

TRUSTSTORE_JKS="truststore.jks"
TRUSTSTORE_P12="truststore.p12"
TRUSTSTORE_PEM="truststore.pem"

echo "Clearing existing Kafka SSL certs..."
cd resources
rm -rf certs
mkdir certs

(
echo "Generating Kafka SSL certs..."
cd certs
# create a new x509 cert, store it in ca-cert and store the key in ca-key
openssl req -new -x509 -keyout ca-key -out ca-cert -days 730 -passout pass:$PASSWORD \
   -subj "/C=US/ST=CA/L=LosAngeles/O=None/OU=None/CN=nat.kafka.ssl"

##
## Trust Store ##
##

# import the cert into the trust store, jks file
keytool -keystore $TRUSTSTORE_JKS -alias CARoot -import -file ca-cert -storepass $PASSWORD -noprompt -storetype pkcs12

# create a p12 for the server keystore
keytool -importkeystore -srckeystore $TRUSTSTORE_JKS -destkeystore $TRUSTSTORE_P12 -srcstoretype JKS -deststoretype PKCS12 -srcstorepass $PASSWORD -deststorepass $PASSWORD -noprompt

# Create a PEM for the server keystore
openssl pkcs12 -in $TRUSTSTORE_P12 -out $TRUSTSTORE_PEM -nodes -passin pass:$PASSWORD

##
## Server Key Store ##
##

# Create the server keystore JKS file
keytool -keystore $SERVER_JKS -alias localhost -validity 730 -genkey -storepass $PASSWORD -keypass $PASSWORD \
  -dname "CN=localhost, OU=None, O=None, L=LosAngeles, ST=California, C=US" -storetype pkcs12 -keyalg RSA -keysize 2048
# Add the cert file to the serer keystore
keytool -keystore $SERVER_JKS -alias localhost -certreq -file server-file -storepass $PASSWORD -noprompt
# sign the cert and save to server-signed
openssl x509 -req -CA ca-cert -CAkey ca-key -in server-file -out server-signed -days 730 -CAcreateserial -passin pass:$PASSWORD
# Add the ca-cert to the server keystore as root
keytool -keystore $SERVER_JKS -alias CARoot -import -file ca-cert -storepass $PASSWORD -noprompt
# Add the signed cert to the server keystore for localhost
keytool -keystore $SERVER_JKS -alias localhost -import -file server-signed -storepass $PASSWORD -noprompt
# create a p12 for the server keystore
keytool -importkeystore -srckeystore $SERVER_JKS -destkeystore $SERVER_P12 -srcstoretype JKS -deststoretype PKCS12 -srcstorepass $PASSWORD -deststorepass $PASSWORD -noprompt
# Create a PEM for the server keystore
openssl pkcs12 -in $SERVER_P12 -out $SERVER_PEM -nodes -passin pass:$PASSWORD

# Export the server cert and convert to .pem
keytool -export -alias localhost -file server-cert.der -keystore $SERVER_JKS -storepass $PASSWORD -noprompt
openssl x509 -inform der -in server-cert.der -out server-cert.pem -passin pass:$PASSWORD
# Export the server key
openssl pkcs12 -in $SERVER_P12 -nodes -nocerts -out server.key -passin pass:$PASSWORD
sed -ne '/-BEGIN PRIVATE KEY-/,/-END PRIVATE KEY-/p' server.key > server-key.pem

##
## Client Key Store ##
##

# Create the client keystore JKS file
keytool -keystore $CLIENT_JKS -alias localhost -validity 730 -genkey -storepass $PASSWORD -keypass $PASSWORD \
  -dname "CN=nat.kafka.client.ssl, OU=None, O=None, L=LosAngeles, ST=California, C=US" -storetype pkcs12 -keyalg RSA -keysize 2048
# Add the cert file to the client keystore
keytool -keystore $CLIENT_JKS -alias localhost -certreq -file client-file -storepass $PASSWORD -noprompt
# sign the cert and save to cert-signed
openssl x509 -req -CA ca-cert -CAkey ca-key -in client-file -out client-signed -days 730 -CAcreateserial -passin pass:$PASSWORD
# Add the ca-cert to the server keystore as root
keytool -keystore $CLIENT_JKS -alias CARoot -import -file ca-cert -storepass $PASSWORD -noprompt
# Add the signed cert to the server keystore for localhost
keytool -keystore $CLIENT_JKS -alias localhost -import -file client-signed -storepass $PASSWORD -noprompt
# create a p12 for the server keystore
keytool -importkeystore -srckeystore $CLIENT_JKS -destkeystore $CLIENT_P12 -srcstoretype JKS -deststoretype PKCS12 -srcstorepass $PASSWORD -deststorepass $PASSWORD -noprompt
# Create a PEM for the server keystore
openssl pkcs12 -in $CLIENT_P12 -out $CLIENT_PEM -nodes -passin pass:$PASSWORD

# Export the client cert and convert to .pem
keytool -export -alias localhost -file client-cert.der -keystore $CLIENT_JKS -storepass $PASSWORD -noprompt
openssl x509 -inform der -in client-cert.der -out client-cert.pem -passin pass:$PASSWORD
# Export the client key
openssl pkcs12 -in $CLIENT_P12 -nodes -nocerts -out client.key -passin pass:$PASSWORD
sed -ne '/-BEGIN PRIVATE KEY-/,/-END PRIVATE KEY-/p' client.key > client-key.pem

# clean up temp files
rm *.der
rm *.p12
rm *.key

# set permissions
chmod +rx *
)