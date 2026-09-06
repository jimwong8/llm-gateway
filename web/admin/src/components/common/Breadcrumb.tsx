import { useEffect } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { navItems } from '../layout/Sidebar'

interface BreadcrumbItem {
  label: string
  path: string
}

export function Breadcrumb() {
  const location = useLocation()
  const navigate = useNavigate()

  const pathSegments = location.pathname.split('/').filter(Boolean)
  
  const items: BreadcrumbItem[] = [
    { label: '首页', path: '/' },
  ]

  let currentPath = ''
  for (const segment of pathSegments) {
    currentPath += `/${segment}`
    const navItem = navItems.find(item => item.path === currentPath)
    if (navItem) {
      items.push({ label: navItem.label, path: currentPath })
    } else if (segment !== 'admin' && segment !== 'ui') {
      items.push({ 
        label: segment.charAt(0).toUpperCase() + segment.slice(1), 
        path: currentPath 
      })
    }
  }

  return (
    <nav className="breadcrumbs" aria-label="面包屑导航">
      {items.map((item, index) => {
        const isLast = index === items.length - 1
        return (
          <span key={item.path} className="breadcrumbs__item">
            {isLast ? (
              <span className="breadcrumbs__current" aria-current="page">
                {item.label}
              </span>
            ) : (
              <>
                <a
                  href={item.path}
                  className="breadcrumbs__link"
                  onClick={(e) => {
                    e.preventDefault()
                    navigate(item.path)
                  }}
                >
                  {item.label}
                </a>
                <span className="breadcrumbs__separator" aria-hidden="true">/</span>
              </>
            )}
          </span>
        )
      })}
    </nav>
  )
}
