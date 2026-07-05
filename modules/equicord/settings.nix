{ lib, parseRules }:
let
  inherit (lib.attrsets) mapAttrs' nameValuePair;

  toUpper =
    string:
    lib.strings.concatStrings [
      (lib.strings.toUpper (builtins.substring 0 1 string))
      (builtins.substring 1 (builtins.stringLength string) string)
    ];

  pluginJsonName = name: parseRules.pluginRenames.${name} or (toUpper name);

  pluginToJson =
    plugin:
    (builtins.removeAttrs plugin [ "enable" ])
    // {
      enabled = plugin.enable or true;
    };

  nixcordConfig = {
    useQuickCss = true;
    plugins = {
      alwaysExpandRoles.enable = true;
      betterGifPicker.enable = true;
      betterSettings.enable = true;
      betterUploadButton.enable = true;
      biggerStreamPreview.enable = true;
      callTimer = {
        enable = true;
        format = "human";
      };
      clearUrls.enable = true;
      crashHandler.enable = true;
      disableCallIdle.enable = true;
      dontRoundMyTimestamps.enable = true;
      favoriteEmojiFirst.enable = true;
      fixCodeblockGap.enable = true;
      fixImagesQuality.enable = true;
      fixYoutubeEmbeds.enable = true;
      forceOwnerCrown.enable = true;
      fullSearchContext.enable = true;
      gifPaste.enable = true;
      greetStickerPicker.enable = true;
      hideMedia.enable = true;
      ignoreActivities = {
        enable = true;
        ignorePlaying = true;
        ignoreListening = true;
        ignoreWatching = true;
        ignoreCompeting = true;
      };
      implicitRelationships.enable = true;
      memberCount.enable = true;
      messageLogger = {
        enable = true;
        collapseDeleted = true;
        ignoreSelf = true;
        ignoreBots = true;
      };
      mutualGroupDms.enable = true;
      newGuildSettings.enable = true;
      noBlockedMessages.enable = true;
      noDevtoolsWarning.enable = true;
      noF1.enable = true;
      noMaskedUrlPaste.enable = true;
      noMosaic.enable = true;
      noPendingCount.enable = true;
      noProfileThemes.enable = true;
      noTypingAnimation.enable = true;
      noUnblockToJump.enable = true;
      onePingPerDm.enable = true;
      pauseInvitesForever.enable = true;
      pictureInPicture.enable = true;
      platformIndicators.enable = true;
      previewMessage.enable = true;
      questify.enable = true;
      readAllNotificationsButton.enable = true;
      relationshipNotifier.enable = true;
      replyTimestamp.enable = true;
      revealAllSpoilers.enable = true;
      serverInfo.enable = true;
      serverListIndicators.enable = true;
      showConnections.enable = true;
      showHiddenThings.enable = true;
      showTimeoutDuration.enable = true;
      silentTyping.enable = true;
      streamerModeOnStream.enable = true;
      themeAttributes.enable = true;
      translate.enable = true;
      typingIndicator.enable = true;
      typingTweaks.enable = true;
      unindent.enable = true;
      unlockedAvatarZoom.enable = true;
      userVoiceShow.enable = true;
      validReply.enable = true;
      validUser.enable = true;
      viewIcons.enable = true;
      voiceChatDoubleClick.enable = true;
      volumeBooster.enable = true;
      youtubeAdblock.enable = true;
    };
  };

  jsonConfig = nixcordConfig // {
    plugins = mapAttrs' (
      name: plugin: nameValuePair (pluginJsonName name) (pluginToJson plugin)
    ) nixcordConfig.plugins;
  };
in
{
  inherit
    jsonConfig
    nixcordConfig
    ;
}
