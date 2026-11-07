import type { RoomCosts } from "../../api/types";
import { COST_COPY } from "./copy";
import { formatCosts } from "./costs";
import styles from "./Organise.module.css";

/** "All rooms £1,352 · Not in a room £9.50 · 2 items without a price" */
export function CostSummaryLine({ costs }: { costs: RoomCosts }) {
  const allRooms = formatCosts(costs.allRooms);
  const notInRoom = formatCosts(costs.unassigned);
  const unpriced =
    costs.allRooms.unpricedItems + costs.unassigned.unpricedItems;
  return (
    <p className={styles.costSummary} aria-label={COST_COPY.summaryLabel}>
      <span>
        {COST_COPY.allRooms}{" "}
        <strong className={styles.cost}>
          {allRooms || COST_COPY.noPrices}
        </strong>
      </span>
      {notInRoom && (
        <span className="muted">
          {COST_COPY.notInRoom} <span className={styles.cost}>{notInRoom}</span>
        </span>
      )}
      {unpriced > 0 && (
        <span className="muted">{COST_COPY.withoutPrice(unpriced)}</span>
      )}
    </p>
  );
}
