set -e
rm -r ../../internal/pump/static/*
touch ../../internal/pump/static/gitkeeper
ng build --configuration "production_moon"
cp -r dist/dashboard/browser/* ../../internal/pump/static/
