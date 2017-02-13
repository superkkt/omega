CREATE TABLE `folder` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint(20) unsigned NOT NULL,
  `parent_id` bigint(20) unsigned NOT NULL DEFAULT 0,
  `name` varchar(64) NOT NULL,
  `type` enum('INBOX', 'DRAFT', 'TRASH', 'SENT', 'FOLDER') NOT NULL default 'INBOX',
  `available` tinyint(1) default true,
  `timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `folder` (`user_id`, `parent_id`, `name`), 
  KEY `user_id` (`user_id`, `type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `folder_history` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint(20) unsigned NOT NULL,
  `folder_id` bigint(20) unsigned NOT NULL,
  `operation` enum('ADD', 'DEL', 'UPDATE') NOT NULL,
  `parent_id` bigint(20) unsigned DEFAULT NULL,
  `name` varchar(64) DEFAULT NULL,
  `timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `user_id` (`user_id`),
  KEY `folder_id` (`folder_id`),
  CONSTRAINT `folder_history_ibfk_1` FOREIGN KEY (`folder_id`) REFERENCES `folder` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `email` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint(20) unsigned NOT NULL,
  `folder_id` bigint(20) unsigned DEFAULT NULL,
  `from` varchar(128) NOT NULL,
  `to` varchar(128) NOT NULL,
  `reply_to` varchar(128) DEFAULT NULL,
  `cc` varchar(128) DEFAULT NULL,
  `subject` varchar(128) NOT NULL,
  `body` text NOT NULL,
  `charset` varchar(128) NOT NULL,
  `read` tinyint(1) NOT NULL DEFAULT FALSE,
  `available` tinyint(1) NOT NULL DEFAULT TRUE,
  `timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `user_id` (`user_id`),
  KEY `folder_id` (`folder_id`),
  CONSTRAINT `email_ibfk_1` FOREIGN KEY (`folder_id`) REFERENCES `folder` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `email_history` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint(20) unsigned NOT NULL,
  `email_id` bigint(20) unsigned NOT NULL,
  `folder_id` bigint(20) unsigned DEFAULT NULL,
  `operation` enum('ADD', 'DEL', 'SEEN') NOT NULL,
  `read` tinyint(1) default NULL,
  `timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `user_id` (`user_id`),
  KEY `email_id` (`email_id`),
  KEY `folder_id` (`folder_id`),
  CONSTRAINT `email_history_ibfk_1` FOREIGN KEY (`email_id`) REFERENCES `email` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `email_history_ibfk_2` FOREIGN KEY (`folder_id`) REFERENCES `folder` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `raw_email` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `email_id` bigint(20) unsigned NOT NULL,
  `data` LONGBLOB NOT NULL,
  PRIMARY KEY (`id`),
  KEY `email_id` (`email_id`),
  CONSTRAINT `raw_email_ibfk_1` FOREIGN KEY (`email_id`) REFERENCES `email` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `attachment` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `email_id` bigint(20) unsigned NOT NULL,
  `content_type` varchar(256) NOT NULL,
  `content_id` varchar(256) NOT NULL,
  `name` varchar(256) NOT NULL,
  `size` int(10) unsigned DEFAULT 0,
  `method` enum('NORMAL', 'INLINE', 'OTHER') DEFAULT 'NORMAL' NOT NULL,
  `order` int(10) unsigned NOT NULL,
  PRIMARY KEY (`id`),
  KEY `email_id` (`email_id`),
  CONSTRAINT `attachment_ibfk_1` FOREIGN KEY (`email_id`) REFERENCES `email` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
