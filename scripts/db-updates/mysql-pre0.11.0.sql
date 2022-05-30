ALTER TABLE `backups` 
	ADD `iid` INT(11) NOT NULL AUTO_INCREMENT FIRST, 
	ADD PRIMARY KEY (`iid`);

ALTER TABLE `guilds` 
	ADD `iid` INT(11) NOT NULL AUTO_INCREMENT FIRST, 
	ADD PRIMARY KEY (`iid`);

ALTER TABLE `permissions` 
	ADD `iid` INT(11) NOT NULL AUTO_INCREMENT FIRST, 
	ADD PRIMARY KEY (`iid`);

ALTER TABLE `reports` 
	ADD `iid` INT(11) NOT NULL AUTO_INCREMENT FIRST, 
	ADD PRIMARY KEY (`iid`);

ALTER TABLE `settings` 
	ADD `iid` INT(11) NOT NULL AUTO_INCREMENT FIRST, 
	ADD PRIMARY KEY (`iid`);

ALTER TABLE `starboard` 
	ADD `iid` INT(11) NOT NULL AUTO_INCREMENT FIRST, 
	ADD PRIMARY KEY (`iid`);

ALTER TABLE `twitchnotify` 
	ADD `iid` INT(11) NOT NULL AUTO_INCREMENT FIRST, 
	ADD PRIMARY KEY (`iid`);

ALTER TABLE `votes` 
	ADD `iid` INT(11) NOT NULL AUTO_INCREMENT FIRST, 
	ADD PRIMARY KEY (`iid`),
	CHANGE `ID` `id` text NOT NULL;