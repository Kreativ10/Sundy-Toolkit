#!/bin/sh
# Run only inside the disposable Alpine container built below.
set -eu
export NO_COLOR=1
mkdir -p /run/openrc
printf 'default\n' > /run/openrc/softlevel
# Docker supplies the network; this fixture satisfies OpenRC's virtual net dependency.
cat > /etc/init.d/net <<'NET'
#!/sbin/openrc-run
description="Container network supplied by Docker"
start() { return 0; }
NET
chmod +x /etc/init.d/net
rc-service net start

printf '1\n2\nopenrc-smoke\n/srv/minecraft/smoke\n25565\n1G\n/usr/lib/jvm/java-21-openjdk/bin/java\ny\ny\ny\n1.21.1\n' | sundy install minecraft
service=sundy-minecraft-openrc-smoke
wait_ready() {
  count=0
  until grep -q 'Done (' /var/lib/sundy/minecraft/openrc-smoke/console.log 2>/dev/null; do
    count=$((count + 1))
    if [ "$count" -gt 120 ]; then
      cat /var/lib/sundy/minecraft/openrc-smoke/console.log
      exit 1
    fi
    sleep 1
  done
}
wait_ready
sundy minecraft status openrc-smoke
(printf 'list\n'; sleep 2; printf ':detach\n') | sundy minecraft console openrc-smoke > /tmp/console-output
grep 'players online' /tmp/console-output
sundy minecraft stop openrc-smoke
test -f /srv/minecraft/smoke/world/level.dat
if rc-service "$service" status; then exit 1; fi
rm /var/lib/sundy/minecraft/openrc-smoke/console.log
sundy minecraft start openrc-smoke
wait_ready
mv /var/lib/sundy/minecraft/openrc-smoke/console.log /tmp/before-restart.log
sundy minecraft restart openrc-smoke
wait_ready
old_pid=$(pgrep -f '^/usr/lib/jvm/java-21-openjdk/bin/java -Xms')
mv /var/lib/sundy/minecraft/openrc-smoke/console.log /tmp/before-crash.log
kill -KILL "$old_pid"
wait_ready
new_pid=$(pgrep -f '^/usr/lib/jvm/java-21-openjdk/bin/java -Xms')
test "$old_pid" != "$new_pid"
sundy minecraft status openrc-smoke
sundy minecraft stop openrc-smoke
if [ -S /run/sundy/minecraft/openrc-smoke.sock ]; then exit 1; fi
# Exercise registration with a relative directory containing spaces, without autostart/restart.
mkdir -p '/srv/minecraft/relative server'
cp /srv/minecraft/smoke/server.jar '/srv/minecraft/relative server/server.jar'
cd /srv/minecraft
printf '1\n1\nexisting-smoke\nrelative server\n25566\n1G\n/usr/lib/jvm/java-21-openjdk/bin/java\nn\nn\ny\nserver.jar\n' | sundy install minecraft
sundy minecraft start existing-smoke
count=0
until grep -q 'Done (' /var/lib/sundy/minecraft/existing-smoke/console.log 2>/dev/null; do
 count=$((count + 1))
 if [ "$count" -gt 120 ]; then exit 1; fi
 sleep 1
done
sundy minecraft stop existing-smoke
printf 'OpenRC real Minecraft creation/registration/start/console/stop/restart passed\n' 
