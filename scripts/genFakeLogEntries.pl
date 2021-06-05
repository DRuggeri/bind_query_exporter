#!/usr/bin/env perl
use strict;
use POSIX qw(strftime);
#Helper to make up fake log entries that look like BIND logs... for local testing

#Flush STDOUT after every print
$| = 1;

my @chars = ('a'..'z');
my @types = ('A', 'AAAA');
my @domains = ('bitnebula.com', 'google.com', 'foobar.com', 'baz.com');
my @clients = ('192.168.0.123', '192.168.0.12', '192.168.0.1');

sub genName {
  return join('', @chars[map {int rand @chars} (1..10)]) . ".com";
}

while (1) {
  my $name = $domains[ rand @domains ];
  my $type = $types[ rand @types ];
  my $client = $clients[ rand @clients ];
  my $t = strftime "%d-%b-%Y %H:%M:%S.000", localtime;
  print "$t queries: info: client \@0xb12ac6d8 $client ($name): query: $name IN $type + (127.0.0.1)\n";
  #05-Jun-2021 07:24:34.669 queries: info: client @0xb12ac6d8 192.168.0.6#39589 (octoprint.org): query: octoprint.org IN AAAA + (192.168.0.53)
  sleep 1;
}
