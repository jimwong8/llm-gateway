import styles from './PageSkeleton.module.css'

export function PageSkeleton() {
  return (
    <div className={styles.skeleton}>
      <div className={styles.header} />
      <div className={styles.content}>
        <div className={styles.line} />
        <div className={styles.line} style={{ width: '60%' }} />
        <div className={styles.line} style={{ width: '80%' }} />
      </div>
    </div>
  )
}
