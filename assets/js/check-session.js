var timeoutGuardBypassAttr = "data-timeout-guard-bypass";
var sessionCheckInProgress = false;
var pendingActions = [];

function findButtonTarget(target) {
  var current = target;

  while (current) {
    var role = current.getAttribute ? (current.getAttribute("role") || "").toLowerCase() : "";
    var tagName = current.tagName ? current.tagName.toLowerCase() : "";
    var type = current.type ? current.type.toLowerCase() : "";

    if (role === "button" || tagName === "button" || (tagName === "input" && type === "submit")) {
      return current;
    }

    current = current.parentElement;
  }

  return null;
}

function isSaveAndSignOut(element) {
  if (!element) {
    return false;
  }

  var buttonText = (element.innerText || element.textContent || element.value || "").toLowerCase().trim();
  return buttonText === "save and sign out";
}

function isSubmitButton(button) {
  if (!button || !button.tagName) {
    return false;
  }

  var tagName = button.tagName.toLowerCase();
  if (tagName === "input") {
    var inputType = (button.type || "").toLowerCase();
    return inputType === "submit" || inputType === "image";
  }

  if (tagName !== "button") {
    return false;
  }

  return (button.getAttribute("type") || "submit").toLowerCase() === "submit";
}

function redirectToTimedOut() {
  window.location.replace("/auth/timed-out");
}

function runAfterSessionCheck(action) {
  pendingActions.push(action);
  if (sessionCheckInProgress) {
    return;
  }

  sessionCheckInProgress = true;

  var xmlHttp = new XMLHttpRequest();
  xmlHttp.onreadystatechange = function() {
    if (xmlHttp.readyState !== 4) {
      return;
    }

    sessionCheckInProgress = false;

    if (xmlHttp.status !== 200) {
      pendingActions = [];
      redirectToTimedOut();
      return;
    }

    var actionsToRun = pendingActions;
    pendingActions = [];

    for (var i = 0; i < actionsToRun.length; i += 1) {
      actionsToRun[i]();
    }
  };

  xmlHttp.onerror = function() {
    sessionCheckInProgress = false;
    pendingActions = [];
    redirectToTimedOut();
  };

  xmlHttp.open("GET", "/auth/logged-in", true);
  xmlHttp.send(null);
}

function replayButtonClick(button) {
  button.setAttribute(timeoutGuardBypassAttr, "true");
  button.click();
  setTimeout(function() {
    button.removeAttribute(timeoutGuardBypassAttr);
  }, 0);
}

function replayFormSubmit(form, submitter) {
  form.setAttribute(timeoutGuardBypassAttr, "true");

  if (typeof form.requestSubmit === "function") {
    if (submitter) {
      form.requestSubmit(submitter);
    } else {
      form.requestSubmit();
    }
  } else {
    form.submit();
  }

  setTimeout(function() {
    form.removeAttribute(timeoutGuardBypassAttr);
  }, 0);
}

window.addEventListener("click", function(event) {
  event = event || window.event;
  var target = event.target || event.srcElement;

  if (target && target.nodeType === 3) {
    target = target.parentElement;
  }

  var buttonTarget = findButtonTarget(target);
  if (!buttonTarget) {
    return;
  }

  if (buttonTarget.getAttribute(timeoutGuardBypassAttr) === "true") {
    return;
  }

  if (isSaveAndSignOut(buttonTarget)) {
    return;
  }

  // Let submit events be guarded by the submit listener so button and form flows stay in sync.
  if (isSubmitButton(buttonTarget)) {
    return;
  }

  event.preventDefault();
  event.stopPropagation();

  runAfterSessionCheck(function() {
    replayButtonClick(buttonTarget);
  });
}, true);

window.addEventListener("submit", function(event) {
  var form = event.target;
  if (!form || form.getAttribute(timeoutGuardBypassAttr) === "true") {
    return;
  }

  var submitter = event.submitter;
  if (isSaveAndSignOut(submitter)) {
    return;
  }

  event.preventDefault();
  event.stopPropagation();

  runAfterSessionCheck(function() {
    replayFormSubmit(form, submitter);
  });
}, true);
