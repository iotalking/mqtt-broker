# 2 millions system-wide
sysctl -w fs.file-max=10485760
sysctl -w fs.nr_open=10485760
echo 2097152 > /proc/sys/fs/nr_open

ulimit -n 1048575



sysctl -w net.core.somaxconn=32768
#表示SYN队列的长度，默认为1024，加大队列长度为8192，可以容纳更多等待连接的网络连接数。
sysctl -w net.ipv4.tcp_max_syn_backlog=16384
# increase the length of the processor input queue
sysctl -w net.core.netdev_max_backlog=16384
#表示用于向外连接的端口范围。缺省情况下很小：32768到61000，改为1000 - 65535。
#（注意：这里不要将最低值设的太低，否则可能会占用掉正常的端口！）
sysctl -w net.ipv4.ip_local_port_range='1024 65535'

#sysctl -w net.ipv4.tcp_tw_recycle=1  #快速回收time_wait的连接
#sysctl -w net.ipv4.tcp_tw_reuse=1
#sysctl -w net.ipv4.tcp_timestamps=1

sysctl -w net.core.rmem_default=262144
sysctl -w net.core.wmem_default=262144
sysctl -w net.core.rmem_max=16777216
sysctl -w net.core.wmem_max=16777216
sysctl -w net.core.optmem_max=16777216

#sysctl -w net.ipv4.tcp_mem='16777216 16777216 16777216'
sysctl -w net.ipv4.tcp_rmem=1024
sysctl -w net.ipv4.tcp_wmem=1024
#表示开启SYN Cookies。当出现SYN等待队列溢出时，启用cookies来处理，可防范少量SYN攻击，默认为0，表示关闭；
sysctl -w net.ipv4.tcp_syncookies=1
#修改系統默认的 TIMEOUT 时间。
sysctl -w net.ipv4.tcp_fin_timeout=30
#表示当keepalive起用的时候，TCP发送keepalive消息的频度。缺省是2小时，改为20分钟。
sysctl -w net.ipv4.tcp_keepalive_time=1200


#额外的，对于内核版本新于**3.7.1**的，我们可以开启tcp_fastopen：
sysctl -w net.ipv4.tcp_fastopen=3

# recommended for hosts with jumbo frames enabled
sysctl -w net.ipv4.tcp_mtu_probing=1

#sysctl -w net.nf_conntrack_max=1000000
#sysctl -w net.netfilter.nf_conntrack_max=1000000
#sysctl -w net.netfilter.nf_conntrack_tcp_timeout_time_wait=30



