# Project roadmap

1. Create a docker-compose.yml (or whatever the standard file name today) with the app itself (`app` service) and postgresql (`db` service). Add .env.example and an actual .env for configs
2. Add user creation and user sessions just like in `../../hrmatch/golang-version/`, BUT user entity should be named "accountant". Create a login page also.
3. Find a way to find out enterpreneur id and name (firm name, i.e. "Viktor Babanov PR Novi Sad") from the loaded PDFs and fill the slip with this data
4. Create an entity "Enterpreneur" and save it into DB along with the related slips. On slip PDF generation also save the current slip state (including amount of money) to the DB.
5. Develop a UI fully based on the Pico CSS: login page, enterpreneur list, enterpreneur page with list of slips, slip page with ability to download a PDF. There also should be a main menu with 2 items (list of enterpreneurs and logout) and footer (links to legal docs, should lead to empty pages for now).

