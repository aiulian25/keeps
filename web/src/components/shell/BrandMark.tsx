import icon24 from "../../assets/brand/keeps-icon-48.png";
import icon80 from "../../assets/brand/keeps-icon-160.png";
import { APPLICATION_NAME } from "../../constants/copy";
import styles from "./Shell.module.css";

/** Supplied Keeps icon plus the name in the UI typeface (approved DEV-01, OD-06). */
export function BrandMark() {
  return (
    <span className={styles.brand}>
      <img src={icon24} alt="" width={24} height={24} />
      <span>{APPLICATION_NAME}</span>
    </span>
  );
}

/** Larger standalone icon for sign-in and setup screens. */
export function BrandIcon() {
  return (
    <img
      className={styles.brandIcon}
      src={icon80}
      alt=""
      width={44}
      height={44}
    />
  );
}
