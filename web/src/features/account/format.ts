import { BROWSER_PATTERNS, SYSTEM_PATTERNS } from "./constants";
import { SESSIONS_COPY, TIME_COPY } from "./copy";

const MILLISECONDS_PER_SECOND = 1000;
const SECONDS_PER_MINUTE = 60;
const MINUTES_PER_HOUR = 60;
const HOURS_PER_DAY = 24;
const DAYS_PER_MONTH = 30;
const DAYS_PER_YEAR = 365;
const SECONDS_PER_HOUR = SECONDS_PER_MINUTE * MINUTES_PER_HOUR;
const SECONDS_PER_DAY = SECONDS_PER_HOUR * HOURS_PER_DAY;

/** Largest unit first, so "3 days ago" wins over "72 hours ago". */
const RELATIVE_UNITS: { unit: Intl.RelativeTimeFormatUnit; seconds: number }[] =
  [
    { unit: "year", seconds: SECONDS_PER_DAY * DAYS_PER_YEAR },
    { unit: "month", seconds: SECONDS_PER_DAY * DAYS_PER_MONTH },
    { unit: "day", seconds: SECONDS_PER_DAY },
    { unit: "hour", seconds: SECONDS_PER_HOUR },
    { unit: "minute", seconds: SECONDS_PER_MINUTE },
  ];

const shortDate = new Intl.DateTimeFormat(undefined, {
  day: "numeric",
  month: "short",
});
const dateTime = new Intl.DateTimeFormat(undefined, {
  dateStyle: "medium",
  timeStyle: "short",
});
const relativeTime = new Intl.RelativeTimeFormat(undefined, {
  numeric: "auto",
});

/** "6 Nov" (in the reader's locale). */
export function formatShortDate(iso: string) {
  return shortDate.format(new Date(iso));
}

/** "15 Jan 2025, 19:42" (in the reader's locale). */
export function formatDateTime(iso: string) {
  return dateTime.format(new Date(iso));
}

/** "2 days ago", "just now". */
export function formatRelativeTime(iso: string) {
  const elapsedSeconds =
    (Date.parse(iso) - Date.now()) / MILLISECONDS_PER_SECOND;
  const match = RELATIVE_UNITS.find(
    ({ seconds }) => Math.abs(elapsedSeconds) >= seconds,
  );
  if (!match) {
    return TIME_COPY.justNow;
  }
  return relativeTime.format(
    Math.round(elapsedSeconds / match.seconds),
    match.unit,
  );
}

/** "Firefox on Linux" from a user agent string. */
export function describeUserAgent(userAgent: string) {
  const browser = BROWSER_PATTERNS.find(({ pattern }) =>
    pattern.test(userAgent),
  )?.name;
  const system = SYSTEM_PATTERNS.find(({ pattern }) =>
    pattern.test(userAgent),
  )?.name;
  if (browser && system) {
    return SESSIONS_COPY.deviceOn(browser, system);
  }
  return browser ?? system ?? SESSIONS_COPY.unknownDevice;
}
