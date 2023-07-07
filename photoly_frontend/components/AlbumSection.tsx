import { BiPhotoAlbum } from "react-icons/bi"
import AlbumHeader from "./AlbumHeader"
import AlbumItem from "./items/AlbumItem"
import {useGetAllAlbumsQuery} from "../redux/api/albumSlice";

const AlbumSection = () => {
  const { data: albums, error, isLoading } = useGetAllAlbumsQuery()
    return (
        <>
            <AlbumHeader headerIcon={BiPhotoAlbum} iconColor="teal.400">
                Your Albums
            </AlbumHeader>
            {isLoading && (<p>loading...</p>)}
            {!isLoading && albums && albums.map((a, id) => (
                <AlbumItem key={id} id={a.gaId} name={a.gaName} coverId={a.coverId} coverColor={a.coverColor} />
            ))}
        </>
    )
}

export default AlbumSection