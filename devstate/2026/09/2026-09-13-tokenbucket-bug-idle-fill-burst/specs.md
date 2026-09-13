# Specs
change: fill-new-key-to-burst
- fold std_go_tokenbucket_allow — idle/new/TTL-expired key starts at burst then consume 1 (FindSpecHost high; candidates std_go_tokenbucket_allow)
- fold std_go_tokenbucket_lua-eval — empty HGETALL seeds tokens=burst last=t (FindSpecHost high; candidates std_go_tokenbucket_lua-eval)
