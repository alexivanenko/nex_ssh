#!/usr/bin/expect

set timeout 20

cd /git_repository_dir/

set cmds [list "git pull" "git push origin master"]

foreach cmd $cmds {
    spawn bash -c $cmd
    expect {
        -nocase "password" {
            exp_send "mypassword\r"
            exp_continue
        }
        eof { wait } ; # at this time the last spawn'ed process has exited
    }
}
