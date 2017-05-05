sudo sysctl -w kern.maxfiles=1048600
sudo sysctl -w kern.maxfilesperproc=1048500
ulimit -n 204800
