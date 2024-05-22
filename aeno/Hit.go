package aeno

import(

)
// Hit represents information about a ray's intersection with a surface.
type Hit struct {
    Point    Vector // The point of intersection in 3D space
    Normal   Vector // The normal vector at the intersection point
    //Material *Material // Material properties at the intersection point (e.g., color, reflectivity)
    // You can add more fields as needed, such as texture coordinates, distance, etc.
}
