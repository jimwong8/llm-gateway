package main

import "fmt"

func main() {
	fmt.Println("tenant scope -> tenant_id=t1 environment=prod scope=tenant")
	fmt.Println("project scope -> tenant_id=t1 environment=prod scope=project project_id=p1")
	fmt.Println("effective precedence -> project override > tenant default")
}
