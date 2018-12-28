# rpmserver

A simple rpm/yum repository server. Usable as a source repo for yum.

Just run a storage directory, the available repositories are defined
by the directories directly within that storage directory. The contents
of the latter are served (by default) under `http://0.0.0.0:8080/rpm/`.
New RPMs can be placed by simply PUTting them at the desired place.
Subdirectories will be created as needed, except at the repository name level.
(I.e.  new repositories must be created with a `mkdir` inside.)

A few seconds after every PUT into a repository `createrepo` is run
inside that repository, to update the index for yum.
