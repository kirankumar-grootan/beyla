#include "vmlinux.h"
#include "common.h"
#include "bpf_helpers.h"
#include "bpf_tracing.h"
#include "bpf_dbg.h"
#include "pid.h"
#include "sockaddr.h"
#include "tcp_info.h"
#include "ringbuf.h"

char __license[] SEC("license") = "Dual MIT/GPL";

// Temporary tracking of accept arguments
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_CONCURRENT_REQUESTS);
    __type(key, u64);
    __type(value, sock_args_t);
} active_accept_args SEC(".maps");

// Temporary tracking of connect arguments
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_CONCURRENT_REQUESTS);
    __type(key, u64);
    __type(value, sock_args_t);
} active_connect_args SEC(".maps");

// Keeps track of active accept or connect connection infos
// From this table we extract the PID of the process and filter
// HTTP calls we are not interested in
struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __type(key, http_connection_info_t);
    __type(value, http_connection_metadata_t); // PID_TID group and connection type
    __uint(max_entries, MAX_CONCURRENT_REQUESTS);
} filtered_connections SEC(".maps");

// Helps us track process names of processes that exited.
// Only used with system wide instrumentation
struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __uint(max_entries, MAX_CONCURRENT_REQUESTS);
    __type(key, u32);
    __type(value, char[16]);
} dead_pids SEC(".maps");

// Keeps track of the tcp sequences we've seen for a connection
// With multiple network interfaces the same sequence can be seen again
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, http_connection_info_t);
    __type(value, u32); // the TCP sequence
    __uint(max_entries, MAX_CONCURRENT_REQUESTS);
} http_tcp_seq SEC(".maps");

// Used by accept to grab the sock details
SEC("kretprobe/sock_alloc")
int BPF_KRETPROBE(kretprobe_sock_alloc, struct socket *sock) {
    u64 id = bpf_get_current_pid_tgid();

    if (!valid_pid(id)) {
        return 0;
    }

    bpf_dbg_printk("=== sock alloc %llx ===", id);

    u64 addr = (u64)sock;

    sock_args_t args = {};

    args.addr = addr;
    args.accept_time = bpf_ktime_get_ns();

    // The socket->sock is not valid until accept finishes, therefore
    // we don't extract ->sock here, we remember the address of socket
    // and parse in sys_accept
    bpf_map_update_elem(&active_accept_args, &id, &args, BPF_ANY);

    return 0;
}

// We tap into both sys_accept and sys_accept4.
// We don't care about the accept entry arguments, since we get only peer information
// we don't have the full picture for the socket.
// 
// Note: A current limitation is that likely we won't capture the first accept request. The
// process may have already reached accept, before the instrumenter has launched.
SEC("kretprobe/sys_accept4")
int BPF_KRETPROBE(kretprobe_sys_accept4, uint fd)
{
    u64 id = bpf_get_current_pid_tgid();

    if (!valid_pid(id)) {
        return 0;
    }

    bpf_dbg_printk("=== accept 4 ret id=%d ===", id);

    // The file descriptor is the value returned from the accept4 syscall.
    // If we got a negative file descriptor we don't have a connection
    if ((int)fd < 0) {
        goto cleanup;
    }

    sock_args_t *args = bpf_map_lookup_elem(&active_accept_args, &id);
    if (!args) {
        bpf_dbg_printk("No sock info %d", id);
        goto cleanup;
    }

    http_connection_info_t info = {};

    parse_accept_socket_info(args, &info);
    sort_connection_info(&info);
    dbg_print_http_connection_info(&info);

    http_connection_metadata_t meta = {};
    meta.id = id;
    meta.flags |= META_HTTP_SRV;

    bpf_map_update_elem(&filtered_connections, &info, &meta, BPF_ANY); // On purpose BPF_ANY, we want to overwrite stale

cleanup:
    bpf_map_delete_elem(&active_accept_args, &id);
    return 0;
}

// Used by connect so that we can grab the sock details
SEC("kprobe/tcp_connect")
int BPF_KPROBE(kprobe_tcp_connect, struct sock *sk) {
    u64 id = bpf_get_current_pid_tgid();

    if (!valid_pid(id)) {
        return 0;
    }

    bpf_dbg_printk("=== tcp connect %llx ===", id);

    u64 addr = (u64)sk;

    sock_args_t args = {};

    args.addr = addr;
    args.accept_time = bpf_ktime_get_ns();

    bpf_map_update_elem(&active_connect_args, &id, &args, BPF_ANY);

    return 0;
}

// We tap into sys_connect so we can track properly the processes doing
// HTTP client calls
SEC("kretprobe/sys_connect")
int BPF_KRETPROBE(kretprobe_sys_connect, int fd)
{
    u64 id = bpf_get_current_pid_tgid();

    if (!valid_pid(id)) {
        return 0;
    }

    bpf_dbg_printk("=== connect ret id=%d, pid=%d ===", id, pid_from_pid_tgid(id));

    // The file descriptor is the value returned from the connect syscall.
    // If we got a negative file descriptor we don't have a connection, unless we are in progress
    if (fd < 0 && (fd != -EINPROGRESS)) {
        goto cleanup;
    }

    sock_args_t *args = bpf_map_lookup_elem(&active_connect_args, &id);
    if (!args) {
        bpf_dbg_printk("No sock info %d", id);
        goto cleanup;
    }

    http_connection_info_t info = {};

    parse_connect_sock_info(args, &info);
    sort_connection_info(&info);
    dbg_print_http_connection_info(&info);

    http_connection_metadata_t meta = {};
    meta.id = id;
    meta.flags |= META_HTTP_SRV;

    bpf_map_update_elem(&filtered_connections, &info, &meta, BPF_ANY); // On purpose BPF_ANY, we want to overwrite stale

cleanup:
    bpf_map_delete_elem(&active_connect_args, &id);
    return 0;
}

SEC("kprobe/sys_exit")
int BPF_KPROBE(kprobe_sys_exit, int status) {
    u64 id = bpf_get_current_pid_tgid();

    if (!valid_pid(id)) {
        return 0;
    }

    u32 pid = pid_from_pid_tgid(id);

    char comm[16];
    bpf_get_current_comm(&comm, sizeof(comm));

    bpf_dbg_printk("=== sys exit id=%d [%s]===", id, comm);

    bpf_map_update_elem(&dead_pids, &pid, &comm, BPF_ANY); // On purpose BPF_ANY, we want to overwrite stale

    return 0;
}

static __always_inline bool tcp_dup(http_connection_info_t *http, protocol_info_t *tcp) {
    u32 *prev_seq = bpf_map_lookup_elem(&http_tcp_seq, http);

    if (prev_seq && (*prev_seq == tcp->seq)) {
        return true;
    }

    bpf_map_update_elem(&http_tcp_seq, http, &tcp->seq, BPF_ANY);
    return false;
}

SEC("socket/http_filter")
int socket__http_filter(struct __sk_buff *skb) {
    protocol_info_t tcp = {};
    http_connection_info_t http = {};

    if (!read_sk_buff(skb, &tcp, &http)) {
        return 0;
    }

    // if we are filtering by application, ignore the packets not for this connection
    http_connection_metadata_t *meta = bpf_map_lookup_elem(&filtered_connections, &http);
    if (!meta) {
        return 0;
    }

    // ignore ACK packets
    if (tcp_ack(&tcp)) {
        return 0;
    }

    // ignore empty packets, unless it's TCP FIN or TCP RST
    if (!tcp_close(&tcp) && tcp_empty(&tcp, skb)) {
        return 0;
    }

    // ignore duplicate sequences
    if (tcp_dup(&http, &tcp)) {
        return 0;
    }

    // we don't support HTTPs yet, quick check for client HTTP calls being SSL, so we don't bother parsing
    if (http.s_port == 443 || http.d_port == 443) {
        return 0;
    }

    bpf_dbg_printk("=== http_filter ===");
    dbg_print_http_connection_info(&http);

    return 0;
}
