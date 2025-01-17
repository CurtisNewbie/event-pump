set -e
rm -r ../../internal/pump/static/*
touch ../../internal/pump/static/gitkeeper
ng build --configuration "production"
cp -r dist/dashboard/browser/* ../../internal/pump/static/
