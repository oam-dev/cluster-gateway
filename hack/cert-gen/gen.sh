SVC_NAME="${SVC_NAME:-kubevela-cluster-gateway}"
SVC_NAMESPACE="${SVC_NAMESPACE:-vela-system}"
OUTPUT_DIR=${OUTPUT_DIR:-./cert}

rm -r $OUTPUT_DIR;
mkdir -p $OUTPUT_DIR;
cd $OUTPUT_DIR;
echo "authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
subjectAltName = @alt_names
[alt_names]
DNS.1 = $SVC_NAME
DNS.2 = $SVC_NAME.$SVC_NAMESPACE.svc" > domain.ext
openssl req -x509 -sha256 -days 3650 -newkey rsa:2048 -keyout ca.key -out ca -nodes -subj '/O=kubevela' \
&& openssl ecparam -name prime256v1 -genkey -noout -out apiserver.key \
&& openssl req -new -key apiserver.key -out apiserver.csr -subj '/O='$SVC_NAME \
&& openssl x509 -req -in apiserver.csr -CA ca -CAkey ca.key -CAcreateserial -extfile domain.ext -out apiserver.crt -days 3650 -sha256

kubectl create secret generic $SVC_NAME -n $SVC_NAMESPACE \
  --from-file=ca=ca \
  --from-file=apiserver.key=apiserver.key \
  --from-file=apiserver.crt=apiserver.crt \
  --dry-run=client -oyaml > $SVC_NAME.yaml

cd ..
mv ./cert/$SVC_NAME.yaml ./
