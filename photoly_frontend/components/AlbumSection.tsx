import { BiPhotoAlbum } from "react-icons/bi"
import AlbumHeader from "./AlbumHeader"
import { useAlbumList } from "./contexts/AlbumContext"
import AlbumItem from "./items/AlbumItem"

const AlbumSection = () => {
    const { albums } = useAlbumList()

    return (
        <>
            <AlbumHeader headerIcon={BiPhotoAlbum} iconColor="teal.400">
                Your Albums
            </AlbumHeader>
            
            {albums.map((a, id) => (
                <AlbumItem key={id} id={a.id} name={a.name} coverId={a.coverId} coverColor={a.coverColor} />
            ))}
        </>
    )
}

export default AlbumSection